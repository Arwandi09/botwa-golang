package plugin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func init() {
	Register(Plugin{
		Command: "brat",
		Desc:    "Membuat stiker teks bergaya brat. Contoh: .brat i love you",
		Run:     BratCommand,
	})
}

func BratCommand(client *whatsmeow.Client, m *events.Message, args []string) {
	text := strings.TrimSpace(strings.Join(args, " "))

	if text == "" {
		reply(client, m, "👉 Contoh: .brat i love you")
		return
	}

	if len(text) > 150 {
		reply(client, m, "🚩 Maksimal 150 karakter.")
		return
	}

	sendReaction(client, m.Info.Chat, m.Info.ID, "🕒")

	randomName := fmt.Sprintf("%d", time.Now().UnixNano())
	tempInput := filepath.Join(os.TempDir(), randomName+"_brat.jpg")
	tempOutput := filepath.Join(os.TempDir(), randomName+"_brat.webp")

	cleanup := func() {
		os.Remove(tempInput)
		os.Remove(tempOutput)
	}

	// Digenerate lewat request langsung ke layanan publik, tanpa Api berbayar.
	url := fmt.Sprintf("https://aqul-brat.hf.space/?text=%s", pathEscapeQuery(text))
	resp, err := http.Get(url)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		sendReaction(client, m.Info.Chat, m.Info.ID, "❌")
		reply(client, m, "❌ Gagal membuat stiker brat.")
		cleanup()
		return
	}
	defer resp.Body.Close()

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		sendReaction(client, m.Info.Chat, m.Info.ID, "❌")
		reply(client, m, "❌ Gagal membuat stiker brat.")
		cleanup()
		return
	}

	if err := os.WriteFile(tempInput, imageData, 0644); err != nil {
		sendReaction(client, m.Info.Chat, m.Info.ID, "❌")
		reply(client, m, "❌ Gagal membuat stiker brat.")
		cleanup()
		return
	}

	ffmpegCmd := exec.Command("ffmpeg", "-y",
		"-i", tempInput,
		"-vf", "scale=512:512:force_original_aspect_ratio=decrease,format=rgba,pad=512:512:-1:-1:color=#00000000",
		"-c:v", "libwebp",
		"-lossless", "0",
		"-quality", "80",
		"-compression_level", "6",
		tempOutput,
	)
	if err := ffmpegCmd.Run(); err != nil {
		sendReaction(client, m.Info.Chat, m.Info.ID, "❌")
		reply(client, m, "❌ Gagal mengonversi stiker. Pastikan ffmpeg terinstal.")
		cleanup()
		return
	}

	stickerData, err := os.ReadFile(tempOutput)
	if err != nil {
		sendReaction(client, m.Info.Chat, m.Info.ID, "❌")
		reply(client, m, "❌ Gagal membuat stiker brat.")
		cleanup()
		return
	}

	ctx := context.Background()
	uploaded, err := client.Upload(ctx, stickerData, whatsmeow.MediaImage)
	if err != nil {
		sendReaction(client, m.Info.Chat, m.Info.ID, "❌")
		reply(client, m, "❌ Gagal mengunggah stiker.")
		cleanup()
		return
	}

	sendBratStickerDirect(client, m.Info.Chat, uploaded)
	sendReaction(client, m.Info.Chat, m.Info.ID, "")
	cleanup()
}

func pathEscapeQuery(s string) string {
	// query escaping sederhana tanpa import net/url terpisah demi konsistensi
	replacer := strings.NewReplacer(
		" ", "%20",
		"\n", "%0A",
		"#", "%23",
		"&", "%26",
		"?", "%3F",
	)
	return replacer.Replace(s)
}

func sendReaction(client *whatsmeow.Client, chat types.JID, msgID string, emoji string) {
	ctx := context.Background()
	remoteJidStr := chat.String()
	fromMe := false

	client.SendMessage(ctx, chat, &waProto.Message{
		ReactionMessage: &waProto.ReactionMessage{
			Key: &waProto.MessageKey{
				RemoteJID: &remoteJidStr,
				FromMe:    &fromMe,
				ID:        &msgID,
			},
			Text:              &emoji,
			SenderTimestampMS: new(int64),
		},
	})
}

func sendBratStickerDirect(client *whatsmeow.Client, chat types.JID, uploaded whatsmeow.UploadResponse) {
	ctx := context.Background()
	mimetype := "image/webp"
	client.SendMessage(ctx, chat, &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
			Mimetype:      &mimetype,
		},
	})
}
