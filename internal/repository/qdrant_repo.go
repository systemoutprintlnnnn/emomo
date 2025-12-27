package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	VectorDimension = 1024
	CollectionName  = "memes"
)

// QdrantRepository handles vector operations with Qdrant
type QdrantRepository struct {
	conn           *grpc.ClientConn
	pointsClient   pb.PointsClient
	collectClient  pb.CollectionsClient
	collectionName string
}

// NewQdrantRepository creates a new QdrantRepository
func NewQdrantRepository(host string, port int, collection string) (*QdrantRepository, error) {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to qdrant: %w", err)
	}

	return &QdrantRepository{
		conn:           conn,
		pointsClient:   pb.NewPointsClient(conn),
		collectClient:  pb.NewCollectionsClient(conn),
		collectionName: collection,
	}, nil
}

// Close closes the gRPC connection
func (r *QdrantRepository) Close() error {
	return r.conn.Close()
}

// EnsureCollection creates the collection if it doesn't exist
func (r *QdrantRepository) EnsureCollection(ctx context.Context) error {
	// Check if collection exists
	_, err := r.collectClient.Get(ctx, &pb.GetCollectionInfoRequest{
		CollectionName: r.collectionName,
	})
	if err == nil {
		return nil // Collection exists
	}

	// Create collection
	_, err = r.collectClient.Create(ctx, &pb.CreateCollection{
		CollectionName: r.collectionName,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: &pb.VectorParams{
					Size:     VectorDimension,
					Distance: pb.Distance_Cosine,
				},
			},
		},
		HnswConfig: &pb.HnswConfigDiff{
			M:              optionalUint64(16),
			EfConstruct:   optionalUint64(128),
			FullScanThreshold: optionalUint64(10000),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}

	return nil
}

func optionalUint64(v uint64) *uint64 {
	return &v
}

// MemePayload represents the payload stored with each vector
type MemePayload struct {
	MemeID         string   `json:"meme_id"`
	SourceType     string   `json:"source_type"`
	Category       string   `json:"category"`
	IsAnimated     bool     `json:"is_animated"`
	Tags           []string `json:"tags"`
	VLMDescription string   `json:"vlm_description"`
	StorageURL     string   `json:"storage_url"`
}

// Upsert inserts or updates a vector with payload
func (r *QdrantRepository) Upsert(ctx context.Context, pointID string, vector []float32, payload *MemePayload) error {
	// Parse UUID
	uid, err := uuid.Parse(pointID)
	if err != nil {
		return fmt.Errorf("invalid point ID: %w", err)
	}

	points := []*pb.PointStruct{
		{
			Id: &pb.PointId{
				PointIdOptions: &pb.PointId_Uuid{
					Uuid: uid.String(),
				},
			},
			Vectors: &pb.Vectors{
				VectorsOptions: &pb.Vectors_Vector{
					Vector: &pb.Vector{
						Data: vector,
					},
				},
			},
			Payload: map[string]*pb.Value{
				"meme_id":         {Kind: &pb.Value_StringValue{StringValue: payload.MemeID}},
				"source_type":     {Kind: &pb.Value_StringValue{StringValue: payload.SourceType}},
				"category":        {Kind: &pb.Value_StringValue{StringValue: payload.Category}},
				"is_animated":     {Kind: &pb.Value_BoolValue{BoolValue: payload.IsAnimated}},
				"vlm_description": {Kind: &pb.Value_StringValue{StringValue: payload.VLMDescription}},
				"storage_url":     {Kind: &pb.Value_StringValue{StringValue: payload.StorageURL}},
				"tags":            tagsToValue(payload.Tags),
			},
		},
	}

	_, err = r.pointsClient.Upsert(ctx, &pb.UpsertPoints{
		CollectionName: r.collectionName,
		Points:         points,
	})
	if err != nil {
		return fmt.Errorf("failed to upsert point: %w", err)
	}

	return nil
}

func tagsToValue(tags []string) *pb.Value {
	values := make([]*pb.Value, len(tags))
	for i, tag := range tags {
		values[i] = &pb.Value{Kind: &pb.Value_StringValue{StringValue: tag}}
	}
	return &pb.Value{
		Kind: &pb.Value_ListValue{
			ListValue: &pb.ListValue{Values: values},
		},
	}
}

// SearchResult represents a search result from Qdrant
type SearchResult struct {
	ID      string
	Score   float32
	Payload *MemePayload
}

// Search performs a vector similarity search
func (r *QdrantRepository) Search(ctx context.Context, vector []float32, topK int, filters *SearchFilters) ([]SearchResult, error) {
	req := &pb.SearchPoints{
		CollectionName: r.collectionName,
		Vector:         vector,
		Limit:          uint64(topK),
		WithPayload: &pb.WithPayloadSelector{
			SelectorOptions: &pb.WithPayloadSelector_Enable{Enable: true},
		},
	}

	// Apply filters if provided
	if filters != nil {
		req.Filter = buildFilter(filters)
	}

	resp, err := r.pointsClient.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	results := make([]SearchResult, len(resp.Result))
	for i, scored := range resp.Result {
		results[i] = SearchResult{
			ID:      scored.Id.GetUuid(),
			Score:   scored.Score,
			Payload: parsePayload(scored.Payload),
		}
	}

	return results, nil
}

// SearchFilters defines optional filters for search
type SearchFilters struct {
	Category   *string
	IsAnimated *bool
	SourceType *string
}

func buildFilter(filters *SearchFilters) *pb.Filter {
	var conditions []*pb.Condition

	if filters.Category != nil && *filters.Category != "" {
		conditions = append(conditions, &pb.Condition{
			ConditionOneOf: &pb.Condition_Field{
				Field: &pb.FieldCondition{
					Key: "category",
					Match: &pb.Match{
						MatchValue: &pb.Match_Keyword{Keyword: *filters.Category},
					},
				},
			},
		})
	}

	if filters.IsAnimated != nil {
		conditions = append(conditions, &pb.Condition{
			ConditionOneOf: &pb.Condition_Field{
				Field: &pb.FieldCondition{
					Key: "is_animated",
					Match: &pb.Match{
						MatchValue: &pb.Match_Boolean{Boolean: *filters.IsAnimated},
					},
				},
			},
		})
	}

	if filters.SourceType != nil && *filters.SourceType != "" {
		conditions = append(conditions, &pb.Condition{
			ConditionOneOf: &pb.Condition_Field{
				Field: &pb.FieldCondition{
					Key: "source_type",
					Match: &pb.Match{
						MatchValue: &pb.Match_Keyword{Keyword: *filters.SourceType},
					},
				},
			},
		})
	}

	if len(conditions) == 0 {
		return nil
	}

	return &pb.Filter{
		Must: conditions,
	}
}

func parsePayload(payload map[string]*pb.Value) *MemePayload {
	if payload == nil {
		return nil
	}

	p := &MemePayload{}
	if v, ok := payload["meme_id"]; ok {
		p.MemeID = v.GetStringValue()
	}
	if v, ok := payload["source_type"]; ok {
		p.SourceType = v.GetStringValue()
	}
	if v, ok := payload["category"]; ok {
		p.Category = v.GetStringValue()
	}
	if v, ok := payload["is_animated"]; ok {
		p.IsAnimated = v.GetBoolValue()
	}
	if v, ok := payload["vlm_description"]; ok {
		p.VLMDescription = v.GetStringValue()
	}
	if v, ok := payload["storage_url"]; ok {
		p.StorageURL = v.GetStringValue()
	}
	if v, ok := payload["tags"]; ok {
		if list := v.GetListValue(); list != nil {
			for _, item := range list.Values {
				p.Tags = append(p.Tags, item.GetStringValue())
			}
		}
	}

	return p
}

// Delete deletes a point by ID
func (r *QdrantRepository) Delete(ctx context.Context, pointID string) error {
	uid, err := uuid.Parse(pointID)
	if err != nil {
		return fmt.Errorf("invalid point ID: %w", err)
	}

	_, err = r.pointsClient.Delete(ctx, &pb.DeletePoints{
		CollectionName: r.collectionName,
		Points: &pb.PointsSelector{
			PointsSelectorOneOf: &pb.PointsSelector_Points{
				Points: &pb.PointsIdsList{
					Ids: []*pb.PointId{
						{PointIdOptions: &pb.PointId_Uuid{Uuid: uid.String()}},
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete point: %w", err)
	}

	return nil
}
