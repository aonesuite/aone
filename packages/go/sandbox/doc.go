//	c, err := sandbox.NewClient(&sandbox.Config{
//	    APIKey: os.Getenv("AONE_API_KEY"),
//	})
//
//	timeout := int32(120)
//	sb, _, err := c.CreateAndWait(ctx, sandbox.CreateParams{
//	    TemplateID: "base",
//	    Timeout:    &timeout,
//	}, sandbox.WithPollInterval(2*time.Second))
//
//	defer sb.Kill(ctx)
//
//
//
//
//
//
//
//
//	result, err := sb.Commands().Run(ctx, "echo hello",
//	    sandbox.WithEnvs(map[string]string{"MY_VAR": "value"}),
//	    sandbox.WithCwd("/tmp"),
//	    sandbox.WithTimeout(5*time.Second),
//	)
//	fmt.Println(result.Stdout)
//
//	handle, err := sb.Commands().Start(ctx, "sleep 30", sandbox.WithTag("bg"))
//	handle.WaitPID(ctx)
//	sb.Commands().Kill(ctx, handle.PID())
//
// （[Commands.SendStdin]）。
//
//	sb.Files().Write(ctx, "/tmp/hello.txt", []byte("Hello!"))
//	content, err := sb.Files().Read(ctx, "/tmp/hello.txt")
//
//	sb.Files().WriteFiles(ctx, []sandbox.WriteEntry{
//	    {Path: "/tmp/a.txt", Data: []byte("content A")},
//	    {Path: "/tmp/b.txt", Data: []byte("content B")},
//	})
//
//	sb.Files().MakeDir(ctx, "/tmp/mydir")
//	entries, err := sb.Files().List(ctx, "/tmp")
//
//	wh, err := sb.Files().WatchDir(ctx, "/tmp/watch", sandbox.WithRecursive(true))
//	for ev := range wh.Events() {
//	    fmt.Printf("event: %s %s\n", ev.Type, ev.Name)
//	}
//
// [Filesystem.Exists]、[Filesystem.GetInfo]、[Filesystem.Rename]、
//
//	ptyHandle, err := sb.Pty().Create(ctx, sandbox.PtySize{Cols: 80, Rows: 24},
//	    sandbox.WithOnStdout(func(data []byte) { fmt.Print(string(data)) }),
//	)
//	sb.Pty().SendInput(ctx, ptyHandle.PID(), []byte("ls -la\n"))
//	sb.Pty().Resize(ctx, ptyHandle.PID(), sandbox.PtySize{Cols: 120, Rows: 40})
//	sb.Pty().Kill(ctx, ptyHandle.PID())
package sandbox
