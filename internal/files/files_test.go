package files

import (
	"os"
	"path/filepath"
	"rbot/internal/db"
	"strings"
	"testing"
)

func TestFilesIndexingAndFinding(t *testing.T) {
	// 1. Create temporary directory structure for testing
	tempDir, err := os.MkdirTemp("", "rbot-files-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	root1 := filepath.Join(tempDir, "root1")
	if err := os.MkdirAll(filepath.Join(root1, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create root1 structure: %v", err)
	}
	
	file1Path := filepath.Join(root1, "file1.txt")
	if err := os.WriteFile(file1Path, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}

	file2Path := filepath.Join(root1, "subdir", "document.pdf")
	if err := os.WriteFile(file2Path, []byte("pdf content"), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	// 2. Initialize temporary DB
	dbPath := filepath.Join(tempDir, "test.db")
	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to init db: %v", err)
	}
	defer database.Close()

	// 3. Test IndexRoots
	allowedRoots := []string{root1}
	err = IndexRoots(database, allowedRoots, nil, nil, 5)
	if err != nil {
		t.Fatalf("IndexRoots failed: %v", err)
	}

	// Verify entries exist in database
	var count int
	err = database.QueryRow("SELECT COUNT(*) FROM path_entries WHERE exists_now = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Query count failed: %v", err)
	}
	// We expect root1, root1/subdir, root1/file1.txt, root1/subdir/document.pdf = 4 entries
	if count != 4 {
		t.Errorf("Expected 4 indexed entries, got %d", count)
	}

	// 4. Test FindFileOrDirectory (exact, FTS5, LIKE, physical walk fallback)
	// Exact match
	found, err := FindFileOrDirectory(database, "file1.txt", allowedRoots, nil)
	if err != nil {
		t.Fatalf("FindFileOrDirectory exact failed: %v", err)
	}
	if found != file1Path {
		t.Errorf("Expected exact find to return %s, got %s", file1Path, found)
	}

	// FTS5/Partial Match
	found, err = FindFileOrDirectory(database, "docu", allowedRoots, nil)
	if err != nil {
		t.Fatalf("FindFileOrDirectory partial failed: %v", err)
	}
	if !strings.Contains(found, "document.pdf") {
		t.Errorf("Expected partial find to return document.pdf path, got %s", found)
	}

	// 5. Test Blocked Paths and Ignore List in indexing
	blockedPath := filepath.Join(root1, "subdir")
	// Clear DB entries (simulate clean start)
	_, _ = database.Exec("DELETE FROM path_entries")
	_, _ = database.Exec("DELETE FROM search_index")

	err = IndexRoots(database, allowedRoots, []string{blockedPath}, nil, 5)
	if err != nil {
		t.Fatalf("IndexRoots with blocked path failed: %v", err)
	}

	// Should not have indexed subdir or document.pdf
	err = database.QueryRow("SELECT COUNT(*) FROM path_entries WHERE exists_now = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Query count failed: %v", err)
	}
	// root1 and root1/file1.txt = 2 entries
	if count != 2 {
		t.Errorf("Expected 2 indexed entries after blocking subdir, got %d", count)
	}

	// Test IgnoreList
	_, _ = database.Exec("DELETE FROM path_entries")
	_, _ = database.Exec("DELETE FROM search_index")
	err = IndexRoots(database, allowedRoots, nil, []string{"document"}, 5)
	if err != nil {
		t.Fatalf("IndexRoots with ignoreList failed: %v", err)
	}

	// Should index root1, root1/subdir, root1/file1.txt but not document.pdf
	err = database.QueryRow("SELECT COUNT(*) FROM path_entries WHERE exists_now = 1").Scan(&count)
	if err != nil {
		t.Fatalf("Query count failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected 3 indexed entries (document.pdf ignored), got %d", count)
	}

	// 6. Test Alias Finding
	// First let's re-index everything to get the entity IDs
	_, _ = database.Exec("DELETE FROM path_entries")
	_, _ = database.Exec("DELETE FROM search_index")
	_ = IndexRoots(database, allowedRoots, nil, nil, 5)

	var entryID int
	err = database.QueryRow("SELECT id FROM path_entries WHERE path = ?", file1Path).Scan(&entryID)
	if err != nil {
		t.Fatalf("Failed to get file1 ID: %v", err)
	}

	_, err = database.Exec("INSERT INTO path_aliases (alias, path_entry_id) VALUES (?, ?)", "my_alias", entryID)
	if err != nil {
		t.Fatalf("Failed to insert alias: %v", err)
	}

	found, err = FindFileOrDirectory(database, "my_alias", allowedRoots, nil)
	if err != nil {
		t.Fatalf("FindFileOrDirectory alias failed: %v", err)
	}
	if found != file1Path {
		t.Errorf("Expected alias find to return %s, got %s", file1Path, found)
	}

	// 7. Test Incremental index and Staleness
	// Delete file2 physically
	if err := os.Remove(file2Path); err != nil {
		t.Fatalf("Failed to delete file2: %v", err)
	}

	// Re-run IndexRoots
	err = IndexRoots(database, allowedRoots, nil, nil, 5)
	if err != nil {
		t.Fatalf("IndexRoots incremental run failed: %v", err)
	}

	// Check if file2 is now marked stale / exists_now = 0
	var existsNow, isStale int
	err = database.QueryRow("SELECT exists_now, is_stale FROM path_entries WHERE path = ?", file2Path).Scan(&existsNow, &isStale)
	if err != nil {
		t.Fatalf("Querying deleted file entry failed: %v", err)
	}
	if existsNow != 0 || isStale != 1 {
		t.Errorf("Expected file2 to be stale (existsNow=0, isStale=1), got existsNow=%d, isStale=%d", existsNow, isStale)
	}

	// Verify check verification on Find
	// If a file is in database but deleted physically, it shouldn't be returned by FindFileOrDirectory and should be marked exists_now = 0
	// Let's delete file1 physically, but not re-run IndexRoots. Let's try to Find file1.
	if err := os.Remove(file1Path); err != nil {
		t.Fatalf("Failed to delete file1: %v", err)
	}

	_, err = FindFileOrDirectory(database, "file1.txt", allowedRoots, nil)
	if err == nil {
		t.Errorf("Expected FindFileOrDirectory to fail since file1.txt was physically deleted")
	}

	err = database.QueryRow("SELECT exists_now, is_stale FROM path_entries WHERE path = ?", file1Path).Scan(&existsNow, &isStale)
	if err != nil {
		t.Fatalf("Querying file1 entry failed: %v", err)
	}
	if existsNow != 0 || isStale != 1 {
		t.Errorf("Expected file1 to be marked stale after search failed, got existsNow=%d, isStale=%d", existsNow, isStale)
	}
}
