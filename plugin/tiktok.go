package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

// ================= INIT =================

func init() {
	Register(Plugin{
		Command: "tiktok",
		Desc:    "Download video TikTok tanpa watermark dari link",
		Run:     tiktokDownload,
	})
}

// ================= DOWNLOAD =================

func tiktokDownload(client *whatsmeow.Client, m *events.Message, args []string) {
	if len(args) == 0 {
		reply(client, m, "Contoh:\n!tiktok https://www.tiktok.com/@user/video/xxxxxxxx")
		return
	}

	url := args[0]
	if !strings.Contains(url, "tiktok.com") {
		reply(client, m, "❌ Link itu bukan link TikTok yang valid.")
		return
	}

	reactProcessing(client, m)

	fileBase := fmt.Sprintf("tiktok_%s", m.Info.ID)
	defer func() {
		matches, _ := filepath.Glob(fileBase + ".*")
		for _, f := range matches {
			os.Remove(f)
		}
	}()

	// yt-dlp secara default mengambil versi TikTok tanpa watermark.
	// --no-part & retries: sama seperti plugin download lain, mengurangi
	// error rename sementara di penyimpanan Android/Termux.
	cmd := exec.Command(
		"yt-dlp",
		"--no-playlist",
		"--no-part",
		"--retries", "10",
		"--fragment-retries", "10",
		"-o", fileBase+".%(ext)s",
		url,
	)

	out, err := cmd.CombinedOutput()

	matches, _ := filepath.Glob(fileBase + ".*")
	if len(matches) == 0 {
		reactError(client, m)
		reply(client, m, "❌ Gagal mengunduh dari TikTok.\nError: "+errString(err)+"\nLog: "+string(out))
		return
	}

	resultFile := matches[0]
	data, err := os.ReadFile(resultFile)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal membaca file hasil unduhan.")
		return
	}

	ext := strings.ToLower(resultFile[strings.LastIndex(resultFile, ".")+1:])
	ctx := context.Background()

	switch ext {
	case "mp4", "mov", "mkv", "webm":
		uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaVideo)
		if errUpload != nil {
			reactError(client, m)
			reply(client, m, "❌ Gagal upload video ke WhatsApp.")
			return
		}
		client.SendMessage(ctx, m.Info.Chat, &waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
				Mimetype:      StringPtr("video/mp4"),
			},
		})

	case "jpg", "jpeg", "png", "webp":
		// Beberapa post TikTok berupa slideshow foto, bukan video.
		uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaImage)
		if errUpload != nil {
			reactError(client, m)
			reply(client, m, "❌ Gagal upload gambar ke WhatsApp.")
			return
		}
		client.SendMessage(ctx, m.Info.Chat, &waProto.Message{
			ImageMessage: &waProto.ImageMessage{
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
				Mimetype:      StringPtr("image/jpeg"),
			},
		})

	default:
		reactError(client, m)
		reply(client, m, "❌ Format file tidak dikenali ("+ext+").")
		return
	}

	reactDone(client, m)
}

// errString mengembalikan pesan error, atau "-" kalau err nil (biasanya
// artinya file target memang tidak ketemu meski proses "berhasil" jalan).
func errString(err error) string {
	if err == nil {
		return "-"
	}
	return err.Error()
}
