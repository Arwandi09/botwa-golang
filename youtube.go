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

// ================= SEARCH =================

func ytSearch(client *whatsmeow.Client, m *events.Message, args []string) {
	if len(args) == 0 {
		reply(client, m, "Contoh:\n!ytsearch lofi hip hop")
		return
	}

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
		reply(client, m, "❌ Gagal melakukan pencarian YouTube")
		return
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 2 {
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

	reply(client, m, result.String())
}

// ================= AUDIO =================

func ytAudio(client *whatsmeow.Client, m *events.Message, args []string) {
	if len(args) == 0 {
		reply(client, m, "Contoh:\n!yta https://youtu.be/xxxx")
		return
	}

	url := args[0]
	file := "yt_audio.mp3"
	defer os.Remove(file)

	cmd := exec.Command(
		"yt-dlp",
		"-x",
		"--audio-format", "mp3",
		"--no-playlist",
		"-o", "yt_audio.%(ext)s",
		url,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		reply(client, m, "❌ yt-dlp error:\n"+string(out))
		return
	}

	if _, err := os.Stat(file); err != nil {
		reply(client, m, "❌ File audio tidak ditemukan setelah download")
		return
	}

	data, err := os.ReadFile(file)
	if err != nil {
		reply(client, m, "❌ Gagal membaca file audio")
		return
	}

	uploaded, err := client.Upload(
		context.Background(),
		data,
		whatsmeow.MediaAudio,
	)
	if err != nil {
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
}

// ================= VIDEO =================

func ytVideo(client *whatsmeow.Client, m *events.Message, args []string) {
	if len(args) == 0 {
		reply(client, m, "Contoh:\n!ytv https://youtu.be/xxxx")
		return
	}

	url := args[0]
	file := "yt_video.mp4"
	defer os.Remove(file)

	cmd := exec.Command(
		"yt-dlp",
		"-f", "bv*[height<=480]+ba/b",
		"--merge-output-format", "mp4",
		"--no-playlist",
		"-o", "yt_video.%(ext)s",
		url,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		reply(client, m, "❌ yt-dlp error:\n"+string(out))
		return
	}

	if _, err := os.Stat(file); err != nil {
		reply(client, m, "❌ File video tidak ditemukan setelah download")
		return
	}

	data, err := os.ReadFile(file)
	if err != nil {
		reply(client, m, "❌ Gagal membaca file video")
		return
	}

	uploaded, err := client.Upload(
		context.Background(),
		data,
		whatsmeow.MediaVideo,
	)
	if err != nil {
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