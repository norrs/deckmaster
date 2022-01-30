package main

import (
	"fmt"
	"image"
	"image/color"
	"time"
)

// TimerWidget is a widget displaying countdown
type TimerWidget struct {
	*BaseWidget

	icon      image.Image
	iconPath  string
	label     string
	fontsize  float64
	colorIdle color.Color
	flatten   bool

	fonts   []string
	colors  []color.Color
	frames  []image.Rectangle

	data    TimerData
}

func (widget *TimerWidget) maybeDrawIdleIconAndTitle(img *image.RGBA) error {
	size := int(widget.dev.Pixels)
	margin := size / 18
	height := size - (margin * 2)
	if widget.label != "" {
		iconsize := int((float64(height) / 3.0) * 2.0)
		bounds := img.Bounds()
		if widget.icon != nil {
			if err := drawImage(img,
				widget.icon,
				iconsize,
				image.Pt(-1, margin)); err != nil {
				return err
			}

			bounds.Min.Y += iconsize + margin
			bounds.Max.Y -= margin
		}

		drawString(img,
			bounds,
			ttfFont,
			widget.label,
			widget.dev.DPI,
			widget.fontsize,
			widget.colorIdle,
			image.Pt(-1, -1))
	} else if widget.icon != nil {
		if err := drawImage(img,
			widget.icon,
			height,
			image.Pt(-1, -1)); err != nil {
			return err
		}
	}
	return nil
}

type TimerData struct {
	deadLine time.Time
	pausedAt time.Time
	timerDuration time.Duration
}

func (data *TimerData) IsRunning() bool {
	return !data.IsPaused() && data.HasDeadline()
}

func (data *TimerData) IsPaused() bool {
	return !data.pausedAt.IsZero()
}

func (data *TimerData) HasDeadline() bool {
	return !data.deadLine.IsZero()
}

func (data *TimerData) Clear() {
	data.deadLine = time.Time{}
	data.pausedAt = time.Time{}
}

// NewTimerWidget returns a new TimerWidget.
func NewTimerWidget(bw *BaseWidget, opts WidgetConfig) (*TimerWidget, error) {
	bw.setInterval(time.Duration(opts.Interval)*time.Millisecond, time.Second/2)
	var fonts  []string
	var icon, label string
	var fontsize float64
	var colorIdle color.Color
	var flatten bool
	var timerDuration time.Duration

	_ = ConfigValue(opts.Config["fontsize"], &fontsize)
	_ = ConfigValue(opts.Config["colorIdle"], &colorIdle)
	_ = ConfigValue(opts.Config["flatten"], &flatten)
	_ = ConfigValue(opts.Config["timerDuration"], &timerDuration)

	_ = ConfigValue(opts.Config["font"], &fonts)
	_ = ConfigValue(opts.Config["icon"], &icon)
	_ = ConfigValue(opts.Config["label"], &label)
	var colors []color.Color
	_ = ConfigValue(opts.Config["colors"], &colors)

	if colorIdle == nil {
		colorIdle = DefaultColor
	}

	if timerDuration.Seconds() < 1 {
		timerDuration, _ = time.ParseDuration("30s")
	}

	layout := NewLayout(int(bw.dev.Pixels))
	frames := layout.FormatLayout([]string{} /* frameReps */, 3)

	for i := 0; i < 3; i++ {
		if len(fonts) < i+1 {
			fonts = append(fonts, "regular")
		}
		if len(colors) < i+1 {
			colors = append(colors, DefaultColor)
		}
	}
	data := TimerData{
		timerDuration: timerDuration,
	}

	widget := &TimerWidget{
		BaseWidget: bw,
		label:      label,
		fontsize:   fontsize,
		colorIdle:  colorIdle,
		flatten:    flatten,

		fonts:   fonts,
		colors:  colors,
		frames:  frames,
		data:    data,
	}

	if icon != "" {
		if err := widget.LoadImage(icon); err != nil {
			return nil, err
		}
	}
	return widget, nil
}

func (w *TimerWidget) RequiresUpdate() bool {

	return w.BaseWidget.RequiresUpdate()
}

// Update renders the widget.
func (w *TimerWidget) Update() error {
	if w.data.IsPaused() {
		return nil
	}
	size := int(w.dev.Pixels)
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	if !w.data.HasDeadline() {
		if err := w.maybeDrawIdleIconAndTitle(img); err != nil {
			return err
		}
	} else {
		w.drawCounter(img)
	}

	return w.render(w.dev, img)
}

// Draws counter to img canvas
func (w *TimerWidget) drawCounter(img *image.RGBA) {
	countdown := w.data.toCountdown()

	skipped := 0
	for i := 0; i < 3; i++ {
		count := countdown[i]
		if count == 0 {
			skipped++
			continue
		}

		// Frames need to be recreated due to number of skipping. Somehow not ideal tho, but works.
		if len(w.frames) != 3-skipped {
			// need to redraw how many frames we need!
			layout := NewLayout(int(w.BaseWidget.dev.Pixels))
			w.frames = layout.FormatLayout([]string{} /* frameReps */, 3-skipped)
		}

		withSkippedIndex := i - skipped

		font := fontByName(w.fonts[withSkippedIndex])
		drawString(img,
			w.frames[withSkippedIndex],
			font,
			fmt.Sprintf("%2d%s", countdown[i], getSuffixInCountdown(i)),
			w.dev.DPI,
			-1,
			w.colors[withSkippedIndex],
			image.Pt(-1, -1))
	}
}

func getSuffixInCountdown(i int) string {
	switch i {
	case 0:
		return "h"
	case 1:
		return "m"
	case 2:
		return "s"
	default:
		return "?"
	}

}

// TriggerAction gets called when a button is pressed.
func (w *TimerWidget) TriggerAction(hold bool) {

	if hold {
		w.data.Clear()
	} else {
		if w.data.IsRunning() {
			w.data.pausedAt = time.Now()
		} else if w.data.IsPaused() && w.data.HasDeadline() {
			pausedDuration := time.Now().Sub(w.data.pausedAt)
			w.data.deadLine = w.data.deadLine.Add(pausedDuration)
			w.data.pausedAt = time.Time{}
		} else {
			w.data.deadLine = time.Now().Add( w.data.timerDuration)
		}
	}
}

func (data *TimerData) toCountdown() []time.Duration {
	elapsedDuration := data.deadLine.Sub(time.Now()).Round(time.Second)
	hours := elapsedDuration / time.Hour // fetch remaining hours
	elapsedDuration -= hours * time.Hour // update elapsedDuration to extract remaining minutes
	minutes := elapsedDuration / time.Minute // fetch remaining minutes
	elapsedDuration -= minutes * time.Minute // update elapsedDuration to extract remaining seconds
	seconds := elapsedDuration / time.Second // fetch remaining seconds
	return []time.Duration{
		hours,
		minutes,
		seconds,
	}
}

// LoadImage loads an image from disk.
func (w *TimerWidget) LoadImage(path string) error {
	path, err := expandPath(w.base, path)
	if err != nil {
		return err
	}
	icon, err := loadImage(path)
	if err != nil {
		return err
	}

	w.SetImage(icon)
	return nil
}

// SetImage updates the widget's icon.
func (w *TimerWidget) SetImage(img image.Image) {
	w.icon = img
	if w.flatten {
		w.icon = flattenImage(w.icon, w.colorIdle)
	}
}
