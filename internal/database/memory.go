package database

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	apiv0 "github.com/modelcontextprotocol/registry/pkg/api/v0"
)

// MemoryDB is an in-memory implementation of the Database interface
type MemoryDB struct {
	entries map[string]*apiv0.ServerJSON // maps record ID (serverID_version) to ServerJSON
	mu      sync.RWMutex
}

func NewMemoryDB() *MemoryDB {
	// Convert input ServerJSON entries to have proper metadata
	serverRecords := make(map[string]*apiv0.ServerJSON)
	return &MemoryDB{
		entries: serverRecords,
	}
}

func (db *MemoryDB) List(
	ctx context.Context,
	filter *ServerFilter,
	cursor string,
	limit int,
) ([]*apiv0.ServerJSON, string, error) {
	if ctx.Err() != nil {
		return nil, "", ctx.Err()
	}

	if limit <= 0 {
		limit = 10 // Default limit
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	// Convert all entries to a slice for pagination
	var allEntries []*apiv0.ServerJSON
	for _, entry := range db.entries {
		allEntries = append(allEntries, entry)
	}

	// Apply filtering and sorting
	filteredEntries := db.filterAndSort(allEntries, filter)

	// Find starting point for cursor-based pagination
	startIdx := 0
	if cursor != "" {
		for i, entry := range filteredEntries {
			if db.getRegistryID(entry) == cursor {
				startIdx = i + 1 // Start after the cursor
				break
			}
		}
	}

	// Apply pagination
	endIdx := min(startIdx+limit, len(filteredEntries))

	var result []*apiv0.ServerJSON
	if startIdx < len(filteredEntries) {
		result = filteredEntries[startIdx:endIdx]
	} else {
		result = []*apiv0.ServerJSON{}
	}

	// Determine next cursor
	nextCursor := ""
	if endIdx < len(filteredEntries) && len(result) > 0 {
		nextCursor = db.getRegistryID(result[len(result)-1])
	}

	return result, nextCursor, nil
}

func (db *MemoryDB) GetByID(ctx context.Context, id string) (*apiv0.ServerJSON, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	db.mu.RLock()
	defer db.mu.RUnlock()

	// Find the latest version with this server ID
	var latestEntry *apiv0.ServerJSON
	for _, entry := range db.entries {
		if entry.Meta != nil && entry.Meta.Official != nil &&
			entry.Meta.Official.ID == id &&
			entry.Meta.Official.IsLatest {
			latestEntry = entry
			break
		}
	}

	if latestEntry != nil {
		// Return a copy of the ServerRecord
		entryCopy := *latestEntry
		return &entryCopy, nil
	}

	return nil, ErrNotFound
}

func (db *MemoryDB) CreateServer(ctx context.Context, server *apiv0.ServerJSON) (*apiv0.ServerJSON, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// Get the server ID from the registry metadata
	if server.Meta == nil || server.Meta.Official == nil {
		return nil, fmt.Errorf("server must have registry metadata with ID")
	}

	serverID := server.Meta.Official.ID
	version := server.Version

	// Generate a unique record ID for this version
	recordID := fmt.Sprintf("%s_%s", serverID, version)

	db.mu.Lock()
	defer db.mu.Unlock()

	// Store the record using the unique record ID
	db.entries[recordID] = server

	return server, nil
}

func (db *MemoryDB) UpdateServer(ctx context.Context, id string, server *apiv0.ServerJSON) (*apiv0.ServerJSON, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Find the specific version to update
	// We need to match both the server ID and version to update the right record
	targetVersion := server.Version
	var oldRecordKey string
	var oldEntry *apiv0.ServerJSON

	// First try to find by exact version match
	targetRecordKey := fmt.Sprintf("%s_%s", id, targetVersion)
	if entry, exists := db.entries[targetRecordKey]; exists {
		if entry.Meta != nil && entry.Meta.Official != nil && entry.Meta.Official.ID == id {
			oldRecordKey = targetRecordKey
			oldEntry = entry
		}
	}

	// If not found by exact key, search for it
	if oldRecordKey == "" {
		for key, entry := range db.entries {
			if entry.Meta != nil && entry.Meta.Official != nil &&
				entry.Meta.Official.ID == id &&
				entry.Version == targetVersion {
				oldRecordKey = key
				oldEntry = entry
				break
			}
		}
	}

	if oldRecordKey == "" {
		return nil, ErrNotFound
	}

	// Ensure the server maintains proper metadata
	if server.Meta == nil {
		server.Meta = &apiv0.ServerMeta{}
	}
	if server.Meta.Official == nil {
		server.Meta.Official = oldEntry.Meta.Official
	}

	// Preserve the server ID (must be consistent)
	server.Meta.Official.ID = id

	// If the version changed, we need to update the record key
	newVersion := server.Version
	oldVersion := oldEntry.Version

	if newVersion != oldVersion {
		// Delete the old record
		delete(db.entries, oldRecordKey)

		// Create new record key with new version
		newRecordKey := fmt.Sprintf("%s_%s", id, newVersion)
		db.entries[newRecordKey] = server
	} else {
		// Version unchanged, just update in place
		db.entries[oldRecordKey] = server
	}

	// Return the updated record
	return server, nil
}

// For an in-memory database, this is a no-op
func (db *MemoryDB) Close() error {
	return nil
}

// filterAndSort applies filtering and sorting to the entries
func (db *MemoryDB) filterAndSort(allEntries []*apiv0.ServerJSON, filter *ServerFilter) []*apiv0.ServerJSON {
	// Apply filtering
	var filteredEntries []*apiv0.ServerJSON
	for _, entry := range allEntries {
		if db.matchesFilter(entry, filter) {
			filteredEntries = append(filteredEntries, entry)
		}
	}

	// Sort by registry metadata ID and version for consistent pagination
	sort.Slice(filteredEntries, func(i, j int) bool {
		iID := db.getRegistryID(filteredEntries[i])
		jID := db.getRegistryID(filteredEntries[j])
		if iID != jID {
			return iID < jID
		}
		// If IDs are the same (same server), sort by version
		return filteredEntries[i].Version < filteredEntries[j].Version
	})

	return filteredEntries
}

// matchesFilter checks if an entry matches the provided filter
//
//nolint:cyclop // Filter matching logic is inherently complex but clear
func (db *MemoryDB) matchesFilter(entry *apiv0.ServerJSON, filter *ServerFilter) bool {
	if filter == nil {
		return true
	}

	// Check name filter
	if filter.Name != nil && entry.Name != *filter.Name {
		return false
	}

	// Check remote URL filter
	if filter.RemoteURL != nil {
		found := false
		for _, remote := range entry.Remotes {
			if remote.URL == *filter.RemoteURL {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check updatedSince filter
	if filter.UpdatedSince != nil {
		if entry.Meta == nil || entry.Meta.Official == nil {
			return false
		}
		if entry.Meta.Official.UpdatedAt.Before(*filter.UpdatedSince) ||
			entry.Meta.Official.UpdatedAt.Equal(*filter.UpdatedSince) {
			return false
		}
	}

	// Check name search filter (substring match)
	if filter.SubstringName != nil {
		// Case-insensitive substring search
		searchLower := strings.ToLower(*filter.SubstringName)
		nameLower := strings.ToLower(entry.Name)
		if !strings.Contains(nameLower, searchLower) {
			return false
		}
	}

	// Check exact version filter
	if filter.Version != nil {
		if entry.Version != *filter.Version {
			return false
		}
	}

	// Check is_latest filter
	if filter.IsLatest != nil {
		if entry.Meta == nil || entry.Meta.Official == nil {
			return false
		}
		if entry.Meta.Official.IsLatest != *filter.IsLatest {
			return false
		}
	}

	return true
}

// getRegistryID safely extracts the registry ID from an entry
func (db *MemoryDB) getRegistryID(entry *apiv0.ServerJSON) string {
	if entry.Meta != nil && entry.Meta.Official != nil {
		return entry.Meta.Official.ID
	}
	return ""
}
