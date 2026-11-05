package model

type StickerBasic struct {
	Type      int `json:"type"`
	CateID    int `json:"cate_id"`
	StickerID int `json:"sticker_id"`
}
type StickerSuggestions struct {
	SuggSticker []StickerBasic `json:"sugg_sticker"`
	SuggGuggy   []StickerBasic `json:"sugg_guggy"`
	SuggGif     []StickerBasic `json:"sugg_gif"`
}

type StickerDetail struct {
	ID               int    `json:"id"`
	CateID           int    `json:"cateId"`
	Type             int    `json:"type"`
	Text             string `json:"text"`
	URI              string `json:"uri"`
	FKey             int    `json:"fkey"`
	Status           int    `json:"status"`
	StickerURL       string `json:"stickerUrl"`
	StickerSpriteURL string `json:"stickerSpriteUrl"`
	StickerWebpURL   any    `json:"stickerWebpUrl"`
	TotalFrames      int    `json:"totalFrames"`
	Duration         int    `json:"duration"`
	EffectID         int    `json:"effectId"`
	Checksum         string `json:"checksum"`
	Ext              int    `json:"ext"`
	Source           int    `json:"source"`
	Fss              any    `json:"fss"`
	FssInfo          any    `json:"fssInfo"`
	Version          int    `json:"version"`
	ExtInfo          any    `json:"extInfo"`
}

type TenorSticker struct {
	ID  string `json:"id"`
	CID int    `json:"cid"`
	EID int    `json:"eid"`
}

type GifMedia struct {
	Width  int    `json:"width"`
	Height int    `json:"height"`
	URL    string `json:"url"`
}

type CategoryDetail struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Desc        string `json:"desc"`
	TotalImage  int    `json:"totalImage"`
	ThumbURL    string `json:"thumbUrl"`
	IconURL     string `json:"iconUrl"`
	IconPreview string `json:"iconPreview"`
	Price       int    `json:"price"`
	Group       int    `json:"group"`
	Status      int    `json:"status"`
	Version     int    `json:"version"`
	ThumbImg    string `json:"thumbImg"`
	Source      string `json:"source"`
	Type        int    `json:"type"`
	SourceURL   string `json:"sourceUrl"`
	Permission  int    `json:"permission"`
	ExpireTime  int64  `json:"expireTime"`
	IsHidden    int    `json:"is_hidden"`
	Order       int    `json:"order"`
}

