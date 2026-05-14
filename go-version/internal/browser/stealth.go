package browser

import (
	"math/rand"
	"time"

	"github.com/playwright-community/playwright-go"
)

// RandomDelay waits for a random duration between min and max milliseconds
func RandomDelay(min, max int) {
	if max <= min {
		time.Sleep(time.Duration(min) * time.Millisecond)
		return
	}
	delay := rand.Intn(max-min) + min
	time.Sleep(time.Duration(delay) * time.Millisecond)
}

// HumanScroll performs a smooth scroll on the page to mimic human behavior
func HumanScroll(page playwright.Page) error {
	for i := 0; i < 3; i++ {
		_, err := page.Evaluate("window.scrollBy(0, Math.floor(Math.random() * 400) + 200)")
		if err != nil {
			return err
		}
		RandomDelay(500, 1200)
	}
	return nil
}

// MouseJiggle moves the mouse slightly to simulate human activity
func MouseJiggle(page playwright.Page) error {
	x := rand.Intn(100) + 100
	y := rand.Intn(100) + 100
	return page.Mouse().Move(float64(x), float64(y))
}