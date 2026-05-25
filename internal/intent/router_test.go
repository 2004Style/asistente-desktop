package intent_test

import (
	"database/sql"
	"testing"
	
	"rbot/internal/db"
	"rbot/internal/intent"
	_ "modernc.org/sqlite"
)

// Set up an in-memory database and populate it with some mock skills
func setupTestDB(t *testing.T) *sql.DB {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open memory db: %v", err)
	}

	_, err = database.Exec(db.Schema)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}
	
	// Add new columns since we altered the schema via migrations in real app
	_, _ = database.Exec("ALTER TABLE skills ADD COLUMN priority INTEGER DEFAULT 0;")
	_, _ = database.Exec("ALTER TABLE skills ADD COLUMN category TEXT;")
	_, _ = database.Exec("ALTER TABLE skills ADD COLUMN exclusive INTEGER DEFAULT 0;")

	// Insert mock skills
	mockSkills := []struct{
		Name string
		Desc string
		JSON string
	}{
		{
			"youtube-media-control",
			"Reproduce música en YouTube",
			`{"name":"youtube-media-control", "voice_triggers":["pon numb", "pon música", "reproduce"], "negative_triggers":["busca información", "investiga"], "risk_level":"low"}`,
		},
		{
			"web-research",
			"Busca información en internet",
			`{"name":"web-research", "voice_triggers":["busca información", "qué es", "investiga"], "negative_triggers":["reproduce", "pon música"], "risk_level":"low"}`,
		},
		{
			"file-reader",
			"Lee archivos de texto",
			`{"name":"file-reader", "voice_triggers":["lee el archivo", "muestra el archivo"], "negative_triggers":["busca web", "youtube"], "risk_level":"low"}`,
		},
		{
			"browser-open",
			"Abre urls en el navegador",
			`{"name":"browser-open", "voice_triggers":["abre youtube", "abre google"], "negative_triggers":["pon numb", "pon música"], "risk_level":"low"}`,
		},
	}

	for _, s := range mockSkills {
		_, err := database.Exec(`
			INSERT INTO skills (name, description, path, skill_md_path, frontmatter_json, enabled)
			VALUES (?, ?, '', '', ?, 1)
		`, s.Name, s.Desc, s.JSON)
		if err != nil {
			t.Fatalf("Failed to insert mock skill: %v", err)
		}
	}

	return database
}

func TestRouterMatches(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	router := intent.NewRouter(database)

	tests := []struct {
		Input       string
		WantSkill   string
		MustNot     string
	}{
		{
			Input:     "pon numb de linkin park",
			WantSkill: "youtube-media-control",
			MustNot:   "web-research",
		},
		{
			Input:     "busca información sobre linkin park",
			WantSkill: "web-research",
			MustNot:   "youtube-media-control",
		},
		{
			Input:     "lee el archivo notas.txt",
			WantSkill: "file-reader",
			MustNot:   "web-research",
		},
		{
			Input:     "abre youtube",
			WantSkill: "browser-open",
			MustNot:   "youtube-media-control",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Input, func(t *testing.T) {
			candidates := router.Match(tc.Input)
			top := intent.Top(candidates)

			if top.SkillName != tc.WantSkill {
				t.Errorf("Para input '%s', se esperaba %s pero ganó %s (Confianza: %.2f)", tc.Input, tc.WantSkill, top.SkillName, top.Confidence)
			}

			if top.SkillName == tc.MustNot {
				t.Errorf("Para input '%s', ganó %s pero estaba prohibido", tc.Input, tc.MustNot)
			}
		})
	}
}
