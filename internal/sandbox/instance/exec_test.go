package instance

import "testing"

func TestBuildExecCommand_QuotesUnsafeArguments(t *testing.T) {
	got := buildExecCommand([]string{
		"python3",
		"-c",
		`print("hello world")`,
		"two words",
		"it's",
	})

	want := `python3 -c 'print("hello world")' 'two words' 'it'"'"'s'`
	if got != want {
		t.Fatalf("buildExecCommand() = %q, want %q", got, want)
	}
}

func TestBuildExecCommand_LeavesShellSafeArgumentsUnquoted(t *testing.T) {
	got := buildExecCommand([]string{
		"echo",
		"plain-text",
		"path/to/file.txt",
		"KEY=value",
	})

	want := `echo plain-text path/to/file.txt KEY=value`
	if got != want {
		t.Fatalf("buildExecCommand() = %q, want %q", got, want)
	}
}
