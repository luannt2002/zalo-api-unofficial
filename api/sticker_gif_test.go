package api

import (
	"testing"

	"github.com/luannt2002/zalo-api-unofficial/session"
)

func TestStickerGifFactories(t *testing.T) {
	sc := session.NewContext()
	sc.SealLogin(session.Seal{
		UID:       "12345",
		UserAgent: "test",
		SecretKey: session.SecretKey("dGVzdC1zZWNyZXQta2V5LTE2Yg=="),
		Settings:  &session.Settings{},
	})

	if _, err := getGiphyGifTrendingFactory(sc, &api{sc: sc}); err != nil {
		t.Fatalf("getGiphyGifTrendingFactory failed: %v", err)
	}
	if _, err := searchGiphyGifFactory(sc, &api{sc: sc}); err != nil {
		t.Fatalf("searchGiphyGifFactory failed: %v", err)
	}
	if _, err := getGiphyGifDictionariesFactory(sc, &api{sc: sc}); err != nil {
		t.Fatalf("getGiphyGifDictionariesFactory failed: %v", err)
	}
	if _, err := getGiphyCategoriesFactory(sc, &api{sc: sc}); err != nil {
		t.Fatalf("getGiphyCategoriesFactory failed: %v", err)
	}
	if _, err := getStickerCategoriesFactory(sc, &api{sc: sc}); err != nil {
		t.Fatalf("getStickerCategoriesFactory failed: %v", err)
	}
	if _, err := getCategoryDetailFactory(sc, &api{sc: sc}); err != nil {
		t.Fatalf("getCategoryDetailFactory failed: %v", err)
	}
	if _, err := getStickerSuggestedKeywordsFactory(sc, &api{sc: sc}); err != nil {
		t.Fatalf("getStickerSuggestedKeywordsFactory failed: %v", err)
	}
	if _, err := getTenorStickerMappingFactory(sc, &api{sc: sc}); err != nil {
		t.Fatalf("getTenorStickerMappingFactory failed: %v", err)
	}
}
