package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDotenv(t *testing.T) {
	content := `# comment line
DOTENV_TEST_A=foo
export DOTENV_TEST_B=bar
DOTENV_TEST_C="quoted value"
DOTENV_TEST_D='single quoted'
DOTENV_TEST_EXISTING=file-value
BAD LINE WITHOUT EQUALS
=empty-key
`
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	for _, key := range []string{"DOTENV_TEST_A", "DOTENV_TEST_B", "DOTENV_TEST_C", "DOTENV_TEST_D"} {
		t.Cleanup(func() { os.Unsetenv(key) })
		os.Unsetenv(key)
	}
	t.Setenv("DOTENV_TEST_EXISTING", "env-value")

	if err := loadDotenv(path); err != nil {
		t.Fatalf("loadDotenv: %v", err)
	}

	cases := map[string]string{
		"DOTENV_TEST_A":        "foo",
		"DOTENV_TEST_B":        "bar",
		"DOTENV_TEST_C":        "quoted value",
		"DOTENV_TEST_D":        "single quoted",
		"DOTENV_TEST_EXISTING": "env-value", // pre-existing env wins
	}
	for key, want := range cases {
		if got := os.Getenv(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestLoadDotenvMissing(t *testing.T) {
	if err := loadDotenv(filepath.Join(t.TempDir(), ".env")); err != nil {
		t.Errorf("missing .env should not error, got %v", err)
	}
}
