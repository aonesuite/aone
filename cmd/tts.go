package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/aonesuite/aone/internal/config"
	internalsbx "github.com/aonesuite/aone/internal/sandbox"
	"github.com/aonesuite/aone/packages/go/tts"
)

type ttsVoicesInfo struct {
	Format string
}

type ttsSpeechInfo struct {
	Text   string
	Voice  string
	Format string
	Speed  float32
	JSON   bool
}

func newTTSCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tts",
		Short:   "Use text-to-speech APIs",
		GroupID: "media",
		Run: func(cmd *cobra.Command, args []string) {
			_ = cmd.Help()
		},
	}
	cmd.AddCommand(newTTSVoicesCmd(), newTTSSpeechCmd())
	return cmd
}

func newTTSVoicesCmd() *cobra.Command {
	info := ttsVoicesInfo{}
	cmd := &cobra.Command{
		Use:     "voices",
		Aliases: []string{"voice"},
		Short:   "List available TTS voices",
		Run: func(cmd *cobra.Command, args []string) {
			ttsVoices(info)
		},
	}
	cmd.Flags().StringVar(&info.Format, "format", "pretty", "output format: pretty or json")
	return cmd
}

func newTTSSpeechCmd() *cobra.Command {
	info := ttsSpeechInfo{}
	cmd := &cobra.Command{
		Use:   "speech",
		Short: "Synthesize text to speech",
		Run: func(cmd *cobra.Command, args []string) {
			ttsSpeech(info)
		},
	}
	cmd.Flags().StringVar(&info.Text, "text", "", "text to synthesize")
	cmd.Flags().StringVar(&info.Voice, "voice", "", "voice ID to use")
	cmd.Flags().StringVar(&info.Format, "audio-format", "", "requested audio format, such as mp3")
	cmd.Flags().Float32Var(&info.Speed, "speed", 0, "relative speech speed where 1.0 is provider default")
	cmd.Flags().BoolVar(&info.JSON, "json", false, "output response as JSON")
	return cmd
}

func newTTSClient() (*tts.Client, error) {
	resolved, err := config.Resolver{}.Resolve()
	if err != nil {
		return nil, err
	}
	return tts.NewClient(&tts.Config{
		APIKey:   resolved.APIKey,
		Endpoint: resolved.Endpoint,
	})
}

func ttsVoices(info ttsVoicesInfo) {
	client, err := newTTSClient()
	if err != nil {
		internalsbx.PrintError("%v", err)
		return
	}
	voices, err := client.ListVoices(context.Background())
	if err != nil {
		internalsbx.PrintError("list voices failed: %v", err)
		return
	}
	if info.Format == internalsbx.FormatJSON {
		internalsbx.PrintJSON(voices)
		return
	}
	if len(voices) == 0 {
		fmt.Println("No voices found")
		return
	}
	tw := internalsbx.NewTable(os.Stdout)
	fmt.Fprintf(tw, "VOICE ID\tNAME\tLANGUAGE\tGENDER\tSCENARIO\n")
	for _, v := range voices {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			v.ID,
			defaultDash(v.Name),
			defaultDash(v.Language),
			defaultDash(v.Gender),
			defaultDash(v.Scenario),
		)
	}
	tw.Flush()
}

func ttsSpeech(info ttsSpeechInfo) {
	if info.Text == "" {
		internalsbx.PrintError("text is required")
		return
	}
	if info.Voice == "" {
		internalsbx.PrintError("voice is required")
		return
	}
	client, err := newTTSClient()
	if err != nil {
		internalsbx.PrintError("%v", err)
		return
	}
	params := tts.SynthesizeParams{
		Text:  info.Text,
		Voice: info.Voice,
	}
	if info.Format != "" {
		params.Format = &info.Format
	}
	if info.Speed > 0 {
		params.Speed = &info.Speed
	}
	resp, err := client.Synthesize(context.Background(), params)
	if err != nil {
		internalsbx.PrintError("synthesize speech failed: %v", err)
		return
	}
	if info.JSON {
		internalsbx.PrintJSON(resp)
		return
	}
	fmt.Printf("Audio URL:   %s\n", resp.AudioURL)
	if resp.DurationMs > 0 {
		fmt.Printf("Duration ms: %d\n", resp.DurationMs)
	}
}

func defaultDash(v string) string {
	if v == "" {
		return "-"
	}
	return v
}

func init() {
	rootCmd.AddCommand(newTTSCmd())
}
