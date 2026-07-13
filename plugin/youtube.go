package plugin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types/events"
	waProto "go.mau.fi/whatsmeow/binary/proto"
)

// ================= INIT =================

func init() {
	Register(Plugin{
		Command: "ytsearch",
		Desc:    "Cari video YouTube",
		Run:     ytSearch,
	})

	Register(Plugin{
		Command: "yta",
		Desc:    "Download audio YouTube (mp3)",
		Run:     ytAudio,
	})

	Register(Plugin{
		Command: "ytv",
		Desc:    "Download video YouTube (mp4 480p)",
		Run:     ytVideo,
	})
}

// ================= HELPER REACTION =================

func BoolPtr(b bool) *bool {
	return &b
}

func reactProcessing(client *whatsmeow.Client, m *events.Message) {
	client.SendMessage(
		context.Background(),
		m.Info.Chat,
		&waProto.Message{
			ReactionMessage: &waProto.ReactionMessage{
				Key: &waProto.MessageKey{
					RemoteJID: StringPtr(m.Info.Chat.String()),
					FromMe:    BoolPtr(false),
					ID:        StringPtr(m.Info.ID),
				},
				Text:              StringPtr("⏳"),
				SenderTimestampMS: nil,
			},
		},
	)
}

func reactDone(client *whatsmeow.Client, m *events.Message) {
	client.SendMessage(
		context.Background(),
		m.Info.Chat,
		&waProto.Message{
			ReactionMessage: &waProto.ReactionMessage{
				Key: &waProto.MessageKey{
					RemoteJID: StringPtr(m.Info.Chat.String()),
					FromMe:    BoolPtr(false),
					ID:        StringPtr(m.Info.ID),
				},
				Text:              StringPtr("✅"),
				SenderTimestampMS: nil,
			},
		},
	)
}

func reactError(client *whatsmeow.Client, m *events.Message) {
	client.SendMessage(
		context.Background(),
		m.Info.Chat,
		&waProto.Message{
			ReactionMessage: &waProto.ReactionMessage{
				Key: &waProto.MessageKey{
					RemoteJID: StringPtr(m.Info.Chat.String()),
					FromMe:    BoolPtr(false),
					ID:        StringPtr(m.Info.ID),
				},
				Text:              StringPtr("❌"),
				SenderTimestampMS: nil,
			},
		},
	)
}

// ================= SEARCH =================

func ytSearch(client *whatsmeow.Client, m *events.Message, args []string) {
	if len(args) == 0 {
		reply(client, m, "Contoh:\n!ytsearch lofi hip hop")
		return
	}

	reactProcessing(client, m)

	query := strings.Join(args, " ")

	cmd := exec.Command(
		"yt-dlp",
		"ytsearch5:"+query,
		"--get-title",
		"--get-id",
	)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal melakukan pencarian YouTube")
		return
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 2 {
		reactError(client, m)
		reply(client, m, "❌ Tidak ada hasil ditemukan")
		return
	}

	var result strings.Builder
	for i := 0; i+1 < len(lines); i += 2 {
		result.WriteString(fmt.Sprintf(
			"%d. %s\nhttps://youtu.be/%s\n\n",
			(i/2)+1,
			lines[i],
			lines[i+1],
		))
	}

	reactDone(client, m)
	reply(client, m, result.String())
}

// ================= AUDIO =================

func ytAudio(client *whatsmeow.Client, m *events.Message, args []string) {
	if len(args) == 0 {
		reply(client, m, "Contoh:\n!yta https://youtu.be/xxxx")
		return
	}

	reactProcessing(client, m)

	url := args[0]
	fileBase := fmt.Sprintf("yt_%s", m.Info.ID)
	fileTarget := fileBase + ".mp3"
	defer os.Remove(fileTarget)

	// Dibuat simpel tanpa banyak parameter format gabungan yang berpotensi error di ffmpeg termux
	cmd := exec.Command(
		"yt-dlp",
		"-x",
		"--audio-format", "mp3",
		"--no-playlist",
		"-o", fileBase+".%(ext)s",
		url,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal mengunduh audio. Pastikan FFmpeg di Termux normal.\nError: "+err.Error()+"\nLog: "+string(out))
		return
	}

	if _, err := os.Stat(fileTarget); err != nil {
		reactError(client, m)
		reply(client, m, "❌ Berhasil diunduh namun format file mp3 tidak ditemukan.")
		return
	}

	data, err := os.ReadFile(fileTarget)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal membaca file audio")
		return
	}

	uploaded, err := client.Upload(
		context.Background(),
		data,
		whatsmeow.MediaAudio,
	)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal upload audio ke WhatsApp")
		return
	}

	client.SendMessage(
		context.Background(),
		m.Info.Chat,
		&waProto.Message{
			AudioMessage: &waProto.AudioMessage{
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
				Mimetype:      StringPtr("audio/mpeg"),
			},
		},
	)

	reactDone(client, m)
}

// ================= VIDEO =================

func ytVideo(client *whatsmeow.Client, m *events.Message, args []string) {
	if len(args) == 0 {
		reply(client, m, "Contoh:\n!ytv https://youtu.be/xxxx")
		return
	}

	reactProcessing(client, m)

	url := args[0]
	fileBase := fmt.Sprintf("yt_%s", m.Info.ID)
	fileTarget := fileBase + ".mp4"
	defer os.Remove(fileTarget)

	// Menggunakan alternatif format instan MP4 standar (bukan gabungan manual) untuk menghindari error link library ffmpeg
	cmd := exec.Command(
		"yt-dlp",
		"-f", "mp4",
		"--no-playlist",
		"-o", fileBase+".%(ext)s",
		url,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal mengunduh video. Pastikan FFmpeg di Termux normal.\nError: "+err.Error()+"\nLog: "+string(out))
		return
	}

	if _, err := os.Stat(fileTarget); err != nil {
		reactError(client, m)
		reply(client, m, "❌ File mp4 hasil unduhan tidak ditemukan.")
		return
	}

	data, err := os.ReadFile(fileTarget)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal membaca file video")
		return
	}

	uploaded, err := client.Upload(
		context.Background(),
		data,
		whatsmeow.MediaVideo,
	)
	if err != nil {
		reactError(client, m)
		reply(client, m, "❌ Gagal upload video ke WhatsApp")
		return
	}

	client.SendMessage(
		context.Background(),
		m.Info.Chat,
		&waProto.Message{
			VideoMessage: &waProto.VideoMessage{
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
				Mimetype:      StringPtr("video/mp4"),
			},
		},
	)

	reactDone(client, m)
}

// ================= HELPER =================

func reply(client *whatsmeow.Client, m *events.Message, text string) {
	client.SendMessage(
		context.Background(),
		m.Info.Chat,
		&waProto.Message{
			Conversation: StringPtr(text),
		},
	)
}
