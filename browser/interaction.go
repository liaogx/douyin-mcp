package browser

import (
	"context"
	"errors"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/proto"
)

// Rod v0.116.2's Element.Click/Input/Screenshot wait for requestAnimationFrame
// through the root page context. A background tab can stop producing frames,
// even after the request has expired. Page.Context also leaves Mouse/Keyboard
// attached to their original context. Keep the tab's event watcher long-lived,
// but send every input command with the current request's bounded context.
const interactionTimeout = 10 * time.Second

func boundedElement(el *rod.Element) (*rod.Element, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(el.GetContext(), interactionTimeout)
	return el.Context(ctx), cancel
}

func visibleStable(el *rod.Element) error {
	// Normal input must not activate a desktop window. The launcher disables
	// background throttling; bounded CDP input does not need OS focus.
	if err := el.WaitVisible(); err != nil {
		return err
	}
	if err := (proto.DOMScrollIntoViewIfNeeded{ObjectID: el.Object.ObjectID}).Call(el); err != nil {
		return err
	}
	// This timer-based Rod method respects the element context and does not
	// use requestAnimationFrame. Never call ScrollIntoView/WaitStableRAF here.
	return el.WaitStable(100 * time.Millisecond)
}

func interactablePoint(el *rod.Element) (*proto.Point, error) {
	if err := visibleStable(el); err != nil {
		return nil, err
	}
	timer := time.NewTicker(100 * time.Millisecond)
	defer timer.Stop()
	for {
		pt, err := el.Interactable()
		if !errors.Is(err, &rod.CoveredError{}) {
			return pt, err
		}
		select {
		case <-el.GetContext().Done():
			return nil, el.GetContext().Err()
		case <-timer.C:
		}
	}
}

func movePointer(el *rod.Element, pt *proto.Point) error {
	buttons := 0
	return (proto.InputDispatchMouseEvent{Type: proto.InputDispatchMouseEventTypeMouseMoved,
		X: pt.X, Y: pt.Y, Button: proto.InputMouseButtonNone, Buttons: &buttons}).Call(el)
}

// Hover uses actual browser input, without bypassing overlays or disabled UI.
func Hover(el *rod.Element) error {
	el, cancel := boundedElement(el)
	defer cancel()
	pt, err := interactablePoint(el)
	if err != nil {
		return err
	}
	return movePointer(el, pt)
}

// Click sends a single trusted left-button press/release pair. It never
// retries the submission, and rechecks the hit target after hover effects.
func Click(el *rod.Element) error {
	el, cancel := boundedElement(el)
	defer cancel()
	if err := el.Wait(rod.Eval(`() => !this.disabled && this.getAttribute('aria-disabled') !== 'true'`)); err != nil {
		return err
	}
	pt, err := interactablePoint(el)
	if err != nil {
		return err
	}
	if err := movePointer(el, pt); err != nil {
		return err
	}
	pt, err = el.Interactable()
	if err != nil {
		return err
	}
	down, up := 1, 0
	if err := (proto.InputDispatchMouseEvent{Type: proto.InputDispatchMouseEventTypeMousePressed,
		X: pt.X, Y: pt.Y, Button: proto.InputMouseButtonLeft, Buttons: &down, ClickCount: 1}).Call(el); err != nil {
		return err
	}
	return (proto.InputDispatchMouseEvent{Type: proto.InputDispatchMouseEventTypeMouseReleased,
		X: pt.X, Y: pt.Y, Button: proto.InputMouseButtonLeft, Buttons: &up, ClickCount: 1}).Call(el)
}

func focus(el *rod.Element) error {
	if _, err := interactablePoint(el); err != nil {
		return err
	}
	if err := el.Wait(rod.Eval(`() => !this.disabled && !this.readOnly && this.getAttribute('aria-disabled') !== 'true'`)); err != nil {
		return err
	}
	return (proto.DOMFocus{ObjectID: el.Object.ObjectID}).Call(el)
}

// InputText inserts Unicode through the browser's normal editing pipeline.
// It preserves the selection prepared by the caller, including rich text.
func InputText(el *rod.Element, value string) error {
	el, cancel := boundedElement(el)
	defer cancel()
	if err := focus(el); err != nil {
		return err
	}
	return (proto.InputInsertText{Text: value}).Call(el)
}

// PressKey is for individual editor keys, not modifier chords or held keys.
func PressKey(el *rod.Element, key input.Key) error {
	if key.Modifier() != 0 {
		return errors.New("不支持保持修饰键按下")
	}
	el, cancel := boundedElement(el)
	defer cancel()
	if err := focus(el); err != nil {
		return err
	}
	if err := key.Encode(proto.InputDispatchKeyEventTypeKeyDown, 0).Call(el); err != nil {
		return err
	}
	return key.Encode(proto.InputDispatchKeyEventTypeKeyUp, 0).Call(el)
}

func SelectAllText(el *rod.Element) error {
	el, cancel := boundedElement(el)
	defer cancel()
	if err := focus(el); err != nil {
		return err
	}
	_, err := el.Eval(`() => {
 if (typeof this.select === 'function') { this.select(); return; }
 const r=document.createRange();r.selectNodeContents(this);
 const s=window.getSelection();s.removeAllRanges();s.addRange(r);
}`)
	return err
}

// ScreenshotPNG captures only the resolved element (used for QR codes), not
// an entire page containing account details. No RAF wait or remote image fetch.
func ScreenshotPNG(el *rod.Element) ([]byte, error) {
	el, cancel := boundedElement(el)
	defer cancel()
	if err := visibleStable(el); err != nil {
		return nil, err
	}
	r, err := el.Eval(`() => {const r=this.getBoundingClientRect();return {x:r.left+scrollX,y:r.top+scrollY,width:r.width,height:r.height,scale:1};}`)
	if err != nil {
		return nil, err
	}
	var clip proto.PageViewport
	if err := r.Value.Unmarshal(&clip); err != nil {
		return nil, err
	}
	if clip.Width <= 0 || clip.Height <= 0 || clip.Width > 4096 || clip.Height > 4096 {
		return nil, errors.New("二维码截图范围无效")
	}
	shot, err := (proto.PageCaptureScreenshot{Format: proto.PageCaptureScreenshotFormatPng, Clip: &clip}).Call(el)
	if err != nil {
		return nil, err
	}
	return shot.Data, nil
}
