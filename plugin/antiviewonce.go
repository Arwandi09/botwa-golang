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
// Perubahan: tidak lagi mengirim reaction agar tidak terlihat, dan pendeteksian dibuat
// lebih agresif dengan pemindaian rekursif terhadap nested/quoted/ephemeral messages.
func AntiViewOncePasif(client *whatsmeow.Client, m *events.Message) bool {
	ctx := context.Background()

	// Debug: print basic message info so we can tell when function is called
	fmt.Printf("[AntiViewOncePasif] triggered: msgID=%s chat=%s fromMe=%v isGroup=%v pushName=%s sender=%s\n",
		m.Info.ID, m.Info.Chat.String(), m.Info.IsFromMe, m.Info.IsGroup, m.Info.PushName, m.Info.Sender.User)

	// 1. Kunci nomor tujuan penerima hasil sadapan (sesuaikan dengan konfigurasi Anda)
	targetJID, errJID := types.ParseJID("6285161098098@s.whatsapp.net")
	if errJID != nil {
		fmt.Printf("[AntiViewOncePasif] parse target JID error: %v\n", errJID)
		return false
	}

	// Jangan proses pesan dari bot sendiri
	if m.Info.IsFromMe || m.Message == nil {
		fmt.Printf("[AntiViewOncePasif] skipped: fromMe=%v or Message==nil\n", m.Info.IsFromMe)
		return false
	}

	// Gunakan scanner rekursif untuk mendeteksi view-once dan media apa saja yang ada
	isViewOnce, imgMsg, vidMsg, audMsg, docMsg := scanMessageForMediaRecursive(m.Message)
	fmt.Printf("[AntiViewOncePasif] scan result: isViewOnce=%v img=%v vid=%v aud=%v doc=%v\n",
		isViewOnce, imgMsg != nil, vidMsg != nil, audMsg != nil, docMsg != nil)

	// Jika tidak terdeteksi view-once atau tidak ada media, abaikan
	if !isViewOnce || (imgMsg == nil && vidMsg == nil && audMsg == nil && docMsg == nil) {
		fmt.Println("[AntiViewOncePasif] not a view-once media or no media objects found")
		return false
	}

	// Siapkan caption info (asal media)
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
		if gi, err := client.GetGroupInfo(ctx, m.Info.Chat); err == nil && gi != nil {
			// beberapa versi wawtsmeow: gunakan field Name
			groupName = gi.Name
		} else {
			groupName = m.Info.Chat.String()
		}
	}

	baseCaptionPrefix := fmt.Sprintf("[RVO PASIF] Dari: %s\nNama: %s\nNomor: %s\nGrup: %s\n", origin, senderName, senderNumber, groupName)

	var data []byte
	var err error

	// helper: fallback tanpa reaction (hanya logging)
	sendFailLog := func(msg string, e error) {
		fmt.Printf("[AntiViewOncePasif] %s: %v\n", msg, e)
	}

	// 5) Download & re-upload based on detected media type
	if imgMsg != nil {
		fmt.Printf("[AntiViewOncePasif] downloading image...\n")
		data, err = client.Download(ctx, imgMsg)
		if err != nil {
			sendFailLog("download image error", err)
			// fallback: forward original message metadata if available
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			return true
		}
		uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaImage)
		if errUpload != nil {
			sendFailLog("upload image error", errUpload)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
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
		return true
	} else if vidMsg != nil {
		fmt.Printf("[AntiViewOncePasif] downloading video...\n")
		data, err = client.Download(ctx, vidMsg)
		if err != nil {
			sendFailLog("download video error", err)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			return true
		}
		uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaVideo)
		if errUpload != nil {
			sendFailLog("upload video error", errUpload)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
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
		return true
	} else if audMsg != nil {
		fmt.Printf("[AntiViewOncePasif] downloading audio...\n")
		data, err = client.Download(ctx, audMsg)
		if err != nil {
			sendFailLog("download audio error", err)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			return true
		}

		// convert to mp3 as before
		fileInput := fmt.Sprintf("temp_pasif_in_%s.ogg", m.Info.ID)
		fileOutput := fmt.Sprintf("temp_pasif_out_%s.mp3", m.Info.ID)

		err = os.WriteFile(fileInput, data, 0644)
		if err != nil {
			sendFailLog("write audio temp error", err)
			return true
		}
		cmd := exec.Command("ffmpeg", "-y", "-i", fileInput, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "128k", fileOutput)
		if errCmd := cmd.Run(); errCmd != nil {
			os.Remove(fileInput)
			sendFailLog("ffmpeg error", errCmd)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			return true
		}
		audioData, errRead := os.ReadFile(fileOutput)
		if errRead != nil {
			os.Remove(fileInput)
			os.Remove(fileOutput)
			sendFailLog("read converted audio error", errRead)
			return true
		}
		uploaded, errUpload := client.Upload(ctx, audioData, whatsmeow.MediaAudio)
		if errUpload != nil {
			os.Remove(fileInput)
			os.Remove(fileOutput)
			sendFailLog("upload audio error", errUpload)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
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
		return true
	} else if docMsg != nil {
		fmt.Printf("[AntiViewOncePasif] downloading document...\n")
		data, err = client.Download(ctx, docMsg)
		if err != nil {
			sendFailLog("download document error", err)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
			return true
		}
		uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaDocument)
		if errUpload != nil {
			sendFailLog("upload document error", errUpload)
			fmt.Printf("[AntiViewOncePasif] fallback: forwarding original message to %s\n", targetJID.String())
			forwardOriginalMessage(client, targetJID, m)
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
		return true
	}

	return true // dicegat, hentikan pemrosesan lebih lanjut di handler
}

// scanMessageForMediaRecursive melakukan pemindaian rekursif pada proto.Message untuk
// menemukan media (image/video/audio/document) dan flag view-once pada level manapun.
func scanMessageForMediaRecursive(msg *proto.Message) (bool, *proto.ImageMessage, *proto.VideoMessage, *proto.AudioMessage, *proto.DocumentMessage) {
	if msg == nil {
		return false, nil, nil, nil, nil
	}

	// check immediate media fields
	if im := msg.GetImageMessage(); im != nil {
		if im.GetViewOnce() {
			return true, im, nil, nil, nil
		}
		// even if not viewonce, continue scanning for wrappers that might mark it viewonce
	}
	if vm := msg.GetVideoMessage(); vm != nil {
		if vm.GetViewOnce() {
			return true, nil, vm, nil, nil
		}
	}
	if am := msg.GetAudioMessage(); am != nil {
		if am.GetViewOnce() {
			return true, nil, nil, am, nil
		}
	}
	if dm := msg.GetDocumentMessage(); dm != nil {
		// Document may not have explicit viewonce flag in some proto versions.
		// We'll treat presence of Document inside a viewonce wrapper as viewonce (handled below).
	}

	// Check wrappers and recurse into inner message
	if v2 := msg.GetViewOnceMessageV2(); v2 != nil && v2.GetMessage() != nil {
		is, im, vm, am, dm := scanMessageForMediaRecursive(v2.GetMessage())
		if is {
			return true, im, vm, am, dm
		}
		// if inner contains media but not flagged, still mark as viewonce because wrapper present
		if im != nil || vm != nil || am != nil || dm != nil {
			return true, im, vm, am, dm
		}
	}
	if v1 := msg.GetViewOnceMessage(); v1 != nil && v1.GetMessage() != nil {
		is, im, vm, am, dm := scanMessageForMediaRecursive(v1.GetMessage())
		if is {
			return true, im, vm, am, dm
		}
		if im != nil || vm != nil || am != nil || dm != nil {
			return true, im, vm, am, dm
		}
	}
	if ext := msg.GetViewOnceMessageV2Extension(); ext != nil && ext.GetMessage() != nil {
		is, im, vm, am, dm := scanMessageForMediaRecursive(ext.GetMessage())
		if is {
			return true, im, vm, am, dm
		}
		if im != nil || vm != nil || am != nil || dm != nil {
			return true, im, vm, am, dm
		}
	}
	if ep := msg.GetEphemeralMessage(); ep != nil && ep.GetMessage() != nil {
		is, im, vm, am, dm := scanMessageForMediaRecursive(ep.GetMessage())
		if is {
			return true, im, vm, am, dm
		}
		if im != nil || vm != nil || am != nil || dm != nil {
			return true, im, vm, am, dm
		}
	}

	// ExtendedTextMessage may contain ContextInfo.QuotedMessage which can embed media
	if ext := msg.GetExtendedTextMessage(); ext != nil && ext.GetContextInfo() != nil {
		if q := ext.GetContextInfo().QuotedMessage; q != nil {
			is, im, vm, am, dm := scanMessageForMediaRecursive(q)
			if is {
				return true, im, vm, am, dm
			}
			if im != nil || vm != nil || am != nil || dm != nil {
				return true, im, vm, am, dm
			}
		}
	}

	// nothing found
	return false, nil, nil, nil, nil
}
