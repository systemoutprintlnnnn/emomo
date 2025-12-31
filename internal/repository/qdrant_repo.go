package repository

import (
	"context"
	"crypto/tls"
	"fmt"

	"github.com/google/uuid"
	pb "github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const (
	defaultVectorDimension = 1024
)

// QdrantConnectionConfig holds configuration for Qdrant connection
type QdrantConnectionConfig struct {
	Host            string
	Port            int
	Collection      string
	APIKey          string // Qdrant Cloud API Key (enables TLS automatically)
	UseTLS          bool   // Explicitly enable TLS without API Key
	VectorDimension int
}

// apiKeyInterceptor creates a unary interceptor that adds API key to metadata
func apiKeyInterceptor(apiKey string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, "api-key", apiKey)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// QdrantRepository handles vector operations with Qdrant
type QdrantRepository struct {
	conn            *grpc.ClientConn
	pointsClient    pb.PointsClient
	collectClient   pb.CollectionsClient
	collectionName  string
	vectorDimension int
}

// NewQdrantRepository creates a new QdrantRepository
// Supports both local Qdrant (insecure) and Qdrant Cloud (TLS + API Key)
func NewQdrantRepository(cfg *QdrantConnectionConfig) (*QdrantRepository, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	vectorDimension := cfg.VectorDimension
	if vectorDimension <= 0 {
		vectorDimension = defaultVectorDimension
	}

	// Build gRPC dial options
	var opts []grpc.DialOption

	// Determine if TLS should be used
	// TLS is enabled if: APIKey is set OR UseTLS is explicitly true
	useTLS := cfg.UseTLS || cfg.APIKey != ""

	if useTLS {
		// Use TLS with system root certificates (TLS 1.3 minimum for Qdrant Cloud)
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS13,
		}
		creds := credentials.NewTLS(tlsConfig)
		opts = append(opts, grpc.WithTransportCredentials(creds))

		// Add API Key authentication if provided (using unary interceptor)
		if cfg.APIKey != "" {
			opts = append(opts, grpc.WithUnaryInterceptor(apiKeyInterceptor(cfg.APIKey)))
		}
	} else {
		// Local mode: no TLS, no authentication
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to qdrant: %w", err)
	}

	return &QdrantRepository{
		conn:            conn,
		pointsClient:    pb.NewPointsClient(conn),
		collectClient:   pb.NewCollectionsClient(conn),
		collectionName:  cfg.Collection,
		vectorDimension: vectorDimension,
	}, nil
}

// Close closes the gRPC connection
func (r *QdrantRepository) Close() error {
	return r.conn.Close()
}

// EnsureCollection creates the collection if it doesn't exist
func (r *QdrantRepository) EnsureCollection(ctx context.Context) error {
	// Check if collection exists
	info, err := r.collectClient.Get(ctx, &pb.GetCollectionInfoRequest{
		CollectionName: r.collectionName,
	})
	if err == nil {
		if size, ok := collectionVectorSize(info.GetResult()); ok {
			if size != uint64(r.vectorDimension) {
				return fmt.Errorf("collection %s has vector size %d, expected %d", r.collectionName, size, r.vectorDimension)
			}
		}
		return nil // Collection exists
	}

	// Create collection
	_, err = r.collectClient.Create(ctx, &pb.CreateCollection{
		CollectionName: r.collectionName,
		VectorsConfig: &pb.VectorsConfig{
			Config: &pb.VectorsConfig_Params{
				Params: &pb.VectorParams{
					Size:     uint64(r.vectorDimension),
					Distance: pb.Distance_Cosine,
				},
			},
		},
		HnswConfig: &pb.HnswConfigDiff{
			M:                 optionalUint64(16),
			EfConstruct:       optionalUint64(128),
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

func collectionVectorSize(info *pb.CollectionInfo) (uint64, bool) {
	if info == nil {
		return 0, false
	}

	config := info.GetConfig()
	if config == nil {
		return 0, false
	}

	params := config.GetParams()
	if params == nil {
		return 0, false
	}

	vectors := params.GetVectorsConfig()
	if vectors == nil {
		return 0, false
	}

	if single := vectors.GetParams(); single != nil {
		if size := single.GetSize(); size > 0 {
			return size, true
		}
	}

	if paramsMap := vectors.GetParamsMap(); paramsMap != nil {
		for _, vectorParams := range paramsMap.GetMap() {
			if vectorParams == nil {
				continue
			}
			if size := vectorParams.GetSize(); size > 0 {
				return size, true
			}
		}
	}

	return 0, false
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
