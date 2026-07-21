package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types/events"
)

func init() {
	Register(Plugin{
		Command: "rvo",
		Desc:    "Download dan kirim kembali media View Once yang di-reply",
		Run:     RvoCommand,
	})
}

func RvoCommand(client *whatsmeow.Client, m *events.Message, args []string) {
	ctx := context.Background()

	// 1. Cek apakah pengguna membalas (reply) sebuah pesan
	if m.Message.GetExtendedTextMessage() == nil || m.Message.GetExtendedTextMessage().GetContextInfo() == nil {
		reply(client, m, "🚩 Silakan balas (reply) pesan View Once yang ingin diunduh.")
		return
	}

	quotedMessage := m.Message.GetExtendedTextMessage().GetContextInfo().QuotedMessage
	if quotedMessage == nil {
		reply(client, m, "🚩 Pesan yang kamu balas tidak valid.")
		return
	}

	// 2. Pencarian Mendalam Struktur Media View Once
	var imgMsg *proto.ImageMessage
	var vidMsg *proto.VideoMessage
	var audMsg *proto.AudioMessage
	var isViewOnce bool

	// Bongkar lapisan View Once (bisa di viewOnceMessage atau viewOnceMessageV2)
	rawMessage := quotedMessage
	if quotedMessage.GetViewOnceMessageV2() != nil {
		rawMessage = quotedMessage.GetViewOnceMessageV2().GetMessage()
		isViewOnce = true
	} else if quotedMessage.GetViewOnceMessage() != nil {
		rawMessage = quotedMessage.GetViewOnceMessage().GetMessage()
		isViewOnce = true
	}

	// Ekstrak tipe medianya jika ada di dalam bungkus viewOnce
	if rawMessage.GetImageMessage() != nil {
		imgMsg = rawMessage.GetImageMessage()
		if imgMsg.GetViewOnce() {
			isViewOnce = true
		}
	} else if rawMessage.GetVideoMessage() != nil {
		vidMsg = rawMessage.GetVideoMessage()
		if vidMsg.GetViewOnce() {
			isViewOnce = true
		}
	} else if rawMessage.GetAudioMessage() != nil {
		audMsg = rawMessage.GetAudioMessage()
		if audMsg.GetViewOnce() {
			isViewOnce = true
		}
	}

	// 3. Validasi Akhir: Pastikan ini benar-benar media View Once
	if !isViewOnce || (imgMsg == nil && vidMsg == nil && audMsg == nil) {
		reply(client, m, "🚩 Itu bukan pesan View Once!")
		return
	}

	reply(client, m, "🕒 Memproses media, mohon tunggu...")

	var data []byte
	var err error

	// 4. Proses Download Media (ctx ditambahkan ke argumen pertama)
	if imgMsg != nil {
		data, err = client.Download(ctx, imgMsg)
		if err == nil {
			uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaImage)
			if errUpload != nil {
				reply(client, m, "❌ Gagal mengunggah gambar ke WhatsApp.")
				return
			}
			client.SendMessage(ctx, m.Info.Chat, &proto.Message{
				ImageMessage: &proto.ImageMessage{
					URL:           &uploaded.URL,
					DirectPath:    &uploaded.DirectPath,
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    &uploaded.FileLength,
					Mimetype:      imgMsg.Mimetype,
					Caption:       imgMsg.Caption,
				},
			})
		}
	} else if vidMsg != nil {
		data, err = client.Download(ctx, vidMsg)
		if err == nil {
			uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaVideo)
			if errUpload != nil {
				reply(client, m, "❌ Gagal mengunggah video ke WhatsApp.")
				return
			}
			client.SendMessage(ctx, m.Info.Chat, &proto.Message{
				VideoMessage: &proto.VideoMessage{
					URL:           &uploaded.URL,
					DirectPath:    &uploaded.DirectPath,
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    &uploaded.FileLength,
					Mimetype:      vidMsg.Mimetype,
					Caption:       vidMsg.Caption,
				},
			})
		}
	} else if audMsg != nil {
		data, err = client.Download(ctx, audMsg)
		if err == nil {
			fileInput := fmt.Sprintf("temp_rvo_in_%s.ogg", m.Info.ID)
			fileOutput := fmt.Sprintf("temp_rvo_out_%s.mp3", m.Info.ID)

			err = os.WriteFile(fileInput, data, 0644)
			if err != nil {
				reply(client, m, "❌ Gagal menulis file audio sementara.")
				return
			}
			defer os.Remove(fileInput)
			defer os.Remove(fileOutput)

			cmd := exec.Command("ffmpeg", "-i", fileInput, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "128k", fileOutput)
			if errCmd := cmd.Run(); errCmd != nil {
				reply(client, m, "❌ Konversi audio gagal. Pastikan ffmpeg terinstall di VPS.")
				return
			}

			audioData, errRead := os.ReadFile(fileOutput)
			if errRead != nil {
				reply(client, m, "❌ Gagal membaca hasil konversi audio.")
				return
			}

			uploaded, errUpload := client.Upload(ctx, audioData, whatsmeow.MediaAudio)
			if errUpload != nil {
				reply(client, m, "❌ Gagal mengunggah audio ke WhatsApp.")
				return
			}

			isPTT := true
			client.SendMessage(ctx, m.Info.Chat, &proto.Message{
				AudioMessage: &proto.AudioMessage{
					URL:           &uploaded.URL,
					DirectPath:    &uploaded.DirectPath,
					MediaKey:      uploaded.MediaKey,
					FileEncSHA256: uploaded.FileEncSHA256,
					FileSHA256:    uploaded.FileSHA256,
					FileLength:    &uploaded.FileLength,
					Mimetype:      StringPtr("audio/mpeg"),
					PTT:           &isPTT, // PERBAIKAN: Huruf besar semua (PTT)
				},
			})
		}
	}

	if err != nil {
		reply(client, m, "❌ Gagal memproses atau mendownload media View Once: "+err.Error())
	}
}
