package plugin

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
)

// HandleViewOnceImage mendownload dan meneruskan image viewonce ke target
func HandleViewOnceImage(client *whatsmeow.Client, ctx context.Context, targetJID types.JID,
	imgMsg *proto.ImageMessage, captionPrefix string, rawMsg *proto.Message) {

	if imgMsg == nil {
		fmt.Printf("[HandleViewOnceImage] imgMsg is nil\n")
		return
	}

	fmt.Printf("[HandleViewOnceImage] downloading image from viewonce...\n")
	data, err := client.Download(ctx, imgMsg)
	if err != nil {
		fmt.Printf("[HandleViewOnceImage] download error: %v, fallback to forward\n", err)
		forwardOriginalMessage(client, targetJID, &fakeMessageEvent{Message: rawMsg})
		return
	}

	fmt.Printf("[HandleViewOnceImage] uploading image...\n")
	uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaImage)
	if errUpload != nil {
		fmt.Printf("[HandleViewOnceImage] upload error: %v, fallback to forward\n", errUpload)
		forwardOriginalMessage(client, targetJID, &fakeMessageEvent{Message: rawMsg})
		return
	}

	caption := captionPrefix + "\n" + imgMsg.GetCaption()
	client.SendMessage(ctx, targetJID, &proto.Message{
		ImageMessage: &proto.ImageMessage{
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
			Mimetype:      imgMsg.Mimetype,
			Caption:       &caption,
		},
	})

	fmt.Printf("[HandleViewOnceImage] image sent successfully\n")
}

// HandleViewOnceVideo mendownload dan meneruskan video viewonce ke target
func HandleViewOnceVideo(client *whatsmeow.Client, ctx context.Context, targetJID types.JID,
	vidMsg *proto.VideoMessage, captionPrefix string, rawMsg *proto.Message) {

	if vidMsg == nil {
		fmt.Printf("[HandleViewOnceVideo] vidMsg is nil\n")
		return
	}

	fmt.Printf("[HandleViewOnceVideo] downloading video from viewonce...\n")
	data, err := client.Download(ctx, vidMsg)
	if err != nil {
		fmt.Printf("[HandleViewOnceVideo] download error: %v, fallback to forward\n", err)
		forwardOriginalMessage(client, targetJID, &fakeMessageEvent{Message: rawMsg})
		return
	}

	fmt.Printf("[HandleViewOnceVideo] uploading video...\n")
	uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaVideo)
	if errUpload != nil {
		fmt.Printf("[HandleViewOnceVideo] upload error: %v, fallback to forward\n", errUpload)
		forwardOriginalMessage(client, targetJID, &fakeMessageEvent{Message: rawMsg})
		return
	}

	caption := captionPrefix + "\n" + vidMsg.GetCaption()
	client.SendMessage(ctx, targetJID, &proto.Message{
		VideoMessage: &proto.VideoMessage{
			URL:           &uploaded.URL,
			DirectPath:    &uploaded.DirectPath,
			MediaKey:      uploaded.MediaKey,
			FileEncSHA256: uploaded.FileEncSHA256,
			FileSHA256:    uploaded.FileSHA256,
			FileLength:    &uploaded.FileLength,
			Mimetype:      vidMsg.Mimetype,
			Caption:       &caption,
		},
	})

	fmt.Printf("[HandleViewOnceVideo] video sent successfully\n")
}

// HandleViewOnceAudio mendownload, convert ke MP3, dan meneruskan audio viewonce ke target
func HandleViewOnceAudio(client *whatsmeow.Client, ctx context.Context, targetJID types.JID,
	audMsg *proto.AudioMessage, captionPrefix string, rawMsg *proto.Message) {

	if audMsg == nil {
		fmt.Printf("[HandleViewOnceAudio] audMsg is nil\n")
		return
	}

	fmt.Printf("[HandleViewOnceAudio] downloading audio from viewonce...\n")
	data, err := client.Download(ctx, audMsg)
	if err != nil {
		fmt.Printf("[HandleViewOnceAudio] download error: %v, fallback to forward\n", err)
		forwardOriginalMessage(client, targetJID, &fakeMessageEvent{Message: rawMsg})
		return
	}

	// Siapkan temp files
	fileInput := fmt.Sprintf("temp_vo_in_%s.ogg", captionPrefix)
	fileOutput := fmt.Sprintf("temp_vo_out_%s.mp3", captionPrefix)

	// Write OGG ke temp
	err = os.WriteFile(fileInput, data, 0644)
	if err != nil {
		fmt.Printf("[HandleViewOnceAudio] write temp error: %v\n", err)
		os.Remove(fileInput)
		return
	}

	// Convert ke MP3 dengan ffmpeg
	cmd := exec.Command("ffmpeg", "-y", "-i", fileInput, "-vn", "-ar", "44100", "-ac", "2", "-b:a", "128k", fileOutput)
	if errCmd := cmd.Run(); errCmd != nil {
		fmt.Printf("[HandleViewOnceAudio] ffmpeg error: %v, fallback to forward\n", errCmd)
		os.Remove(fileInput)
		forwardOriginalMessage(client, targetJID, &fakeMessageEvent{Message: rawMsg})
		return
	}

	// Baca MP3 hasil konversi
	audioData, errRead := os.ReadFile(fileOutput)
	if errRead != nil {
		fmt.Printf("[HandleViewOnceAudio] read converted audio error: %v\n", errRead)
		os.Remove(fileInput)
		os.Remove(fileOutput)
		return
	}

	// Upload audio
	fmt.Printf("[HandleViewOnceAudio] uploading audio...\n")
	uploaded, errUpload := client.Upload(ctx, audioData, whatsmeow.MediaAudio)
	if errUpload != nil {
		fmt.Printf("[HandleViewOnceAudio] upload error: %v, fallback to forward\n", errUpload)
		os.Remove(fileInput)
		os.Remove(fileOutput)
		forwardOriginalMessage(client, targetJID, &fakeMessageEvent{Message: rawMsg})
		return
	}

	// Send dengan PTT flag
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

	// Cleanup
	os.Remove(fileInput)
	os.Remove(fileOutput)

	fmt.Printf("[HandleViewOnceAudio] audio sent successfully\n")
}

// HandleViewOnceDocument mendownload dan meneruskan document viewonce ke target
func HandleViewOnceDocument(client *whatsmeow.Client, ctx context.Context, targetJID types.JID,
	docMsg *proto.DocumentMessage, captionPrefix string, rawMsg *proto.Message) {

	if docMsg == nil {
		fmt.Printf("[HandleViewOnceDocument] docMsg is nil\n")
		return
	}

	fmt.Printf("[HandleViewOnceDocument] downloading document from viewonce...\n")
	data, err := client.Download(ctx, docMsg)
	if err != nil {
		fmt.Printf("[HandleViewOnceDocument] download error: %v, fallback to forward\n", err)
		forwardOriginalMessage(client, targetJID, &fakeMessageEvent{Message: rawMsg})
		return
	}

	fmt.Printf("[HandleViewOnceDocument] uploading document...\n")
	uploaded, errUpload := client.Upload(ctx, data, whatsmeow.MediaDocument)
	if errUpload != nil {
		fmt.Printf("[HandleViewOnceDocument] upload error: %v, fallback to forward\n", errUpload)
		forwardOriginalMessage(client, targetJID, &fakeMessageEvent{Message: rawMsg})
		return
	}

	caption := captionPrefix + "\n" + docMsg.GetFileName()
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
			Caption:       &caption,
		},
	})

	fmt.Printf("[HandleViewOnceDocument] document sent successfully\n")
}

// fakeMessageEvent adalah helper untuk kompatibilitas dengan forwardOriginalMessage
// yang expect *events.Message
type fakeMessageEvent struct {
	Message *proto.Message
}
