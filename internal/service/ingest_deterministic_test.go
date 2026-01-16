package service

import (
	"testing"
)

// TestGenerateDeterministicPointID verifies that the same input always produces the same UUID
func TestGenerateDeterministicPointID(t *testing.T) {
	testCases := []struct {
		name       string
		md5Hash    string
		collection string
	}{
		{
			name:       "basic test",
			md5Hash:    "abc123def456",
			collection: "emomo",
		},
		{
			name:       "different collection",
			md5Hash:    "abc123def456",
			collection: "emomo-test",
		},
		{
			name:       "different md5",
			md5Hash:    "xyz789uvw012",
			collection: "emomo",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Generate UUID multiple times
			uuid1 := generateDeterministicPointID(tc.md5Hash, tc.collection)
			uuid2 := generateDeterministicPointID(tc.md5Hash, tc.collection)
			uuid3 := generateDeterministicPointID(tc.md5Hash, tc.collection)

			// All should be identical
			if uuid1 != uuid2 {
				t.Errorf("UUID mismatch: first=%s, second=%s", uuid1, uuid2)
			}
			if uuid1 != uuid3 {
				t.Errorf("UUID mismatch: first=%s, third=%s", uuid1, uuid3)
			}

			// Should be a valid UUID format
			if len(uuid1) != 36 {
				t.Errorf("Invalid UUID length: got %d, want 36", len(uuid1))
			}
		})
	}
}

// TestGenerateDeterministicPointIDUniqueness verifies that different inputs produce different UUIDs
func TestGenerateDeterministicPointIDUniqueness(t *testing.T) {
	uuid1 := generateDeterministicPointID("abc123", "emomo")
	uuid2 := generateDeterministicPointID("def456", "emomo")
	uuid3 := generateDeterministicPointID("abc123", "emomo-test")

	if uuid1 == uuid2 {
		t.Errorf("Different MD5 hashes should produce different UUIDs: %s == %s", uuid1, uuid2)
	}
	if uuid1 == uuid3 {
		t.Errorf("Different collections should produce different UUIDs: %s == %s", uuid1, uuid3)
	}
	if uuid2 == uuid3 {
		t.Errorf("Different inputs should produce different UUIDs: %s == %s", uuid2, uuid3)
	}
}
