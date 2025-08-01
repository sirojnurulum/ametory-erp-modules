package whatsmeow_client

type WaMessage struct {
	JID          string  `json:"jid"`
	Text         string  `json:"text"`
	FileType     string  `json:"file_type"`
	FileUrl      string  `json:"file_url"`
	To           string  `json:"to"`
	IsGroup      bool    `json:"is_group"`
	RefID        *string `json:"ref_id"`
	RefFrom      *string `json:"ref_from"`
	RefText      *string `json:"ref_text"`
	ChatPresence string  `json:"chat_presence"`
}
