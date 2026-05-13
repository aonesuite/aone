// Package sandbox provides the Go SDK for creating and operating aone
// sandboxes.
//
// A Client talks to the sandbox control plane. It can create sandboxes, connect
// to existing sandboxes, list running sandboxes, manage templates, snapshots,
// and volumes, and inspect logs or metrics. A Sandbox value represents one
// running sandbox and exposes helpers for commands, files, Git, PTY sessions,
// snapshots, and lifecycle operations.
//
// The SDK reads AONE_API_KEY when Config.APIKey is empty. A custom endpoint can
// be supplied through Config.Endpoint or AONE_SANDBOX_API_URL.
//
// Basic usage:
//
//	ctx := context.Background()
//
//	client, err := sandbox.NewClient(&sandbox.Config{
//		APIKey: os.Getenv("AONE_API_KEY"),
//	})
//	if err != nil {
//		return err
//	}
//
//	timeout := int32(120)
//	sb, _, err := client.CreateAndWait(ctx, sandbox.CreateParams{
//		TemplateID: "base",
//		Timeout:    &timeout,
//	}, sandbox.WithPollInterval(2*time.Second))
//	if err != nil {
//		return err
//	}
//	defer sb.Kill(ctx)
//
//	result, err := sb.Commands().Run(ctx, "echo hello",
//		sandbox.WithCwd("/tmp"),
//		sandbox.WithTimeout(5*time.Second),
//	)
//	if err != nil {
//		return err
//	}
//	fmt.Print(result.Stdout)
//
//	err = sb.Files().Write(ctx, "/tmp/hello.txt", []byte("hello\n"))
//	if err != nil {
//		return err
//	}
//
//	status, err := sb.Git().Status(ctx, "/workspace/repo")
//	if err != nil {
//		return err
//	}
//	if status.HasChanges() {
//		fmt.Println("repository has local changes")
//	}
package sandbox
