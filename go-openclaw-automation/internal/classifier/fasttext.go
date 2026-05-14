package classifier

import (
	"path/filepath"
	"strings"
	"sync"
	"log"

	"github.com/myagmartseren/fasttext_golang"
)

var (
	nativeModel *fasttext.FastText
	modelOnce   sync.Once
)

func getNativeModel(openclawRoot string) *fasttext.FastText {
	modelOnce.Do(func() {
		nativeModel = fasttext.NewFasttext()
		modelPath := filepath.Join(openclawRoot, "execution", "models", "social-hiring.ftz")
		err := nativeModel.LoadModel(modelPath)
		if err != nil {
			log.Printf("⚠️ Failed to load native fastText model: %v", err)
			nativeModel = nil
		} else {
			log.Printf("✅ Native fastText model loaded successfully from %s", modelPath)
		}
	})
	return nativeModel
}

func ClassifyWithFastTextNative(openclawRoot string, text string) *ClassificationResult {
	model := getNativeModel(openclawRoot)
	if model == nil {
		return nil
	}

	res, err := model.Predict(strings.ReplaceAll(text, "\n", " "))
	if err != nil || res == "" {
		return nil
	}

	label := strings.ReplaceAll(strings.TrimSpace(res), "__label__", "")
	
	// The CGO wrapper currently returns only the top label without probability.
	// We assume high confidence if the model returns it.
	return &ClassificationResult{
		Label:      label,
		IsHiring:   label == "hiring",
		Confidence: 0.90, // default high confidence since it's the top prediction
		Margin:     0.50, // default margin
		Source:     "fasttext-native",
	}
}

func ClassifySocialHiringPost(seedModel *Model, openclawRoot string, text string) ClassificationResult {
	ftResult := ClassifyWithFastTextNative(openclawRoot, text)
	if ftResult != nil {
		return *ftResult
	}
	return ClassifyWithSeedModel(seedModel, text)
}
