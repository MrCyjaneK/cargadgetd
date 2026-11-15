package shell

// MapTouch converts AA InputEvent action to plugin ABI: 0 up, 1 down, 2 move.
func MapTouch(aaAction int) (pluginAction int, ok bool) {
	switch aaAction {
	case 0, 3, 5: // Press, Cancel-as-down (some HUs), PointerDown
		return 1, true
	case 1, 4, 6: // Release, Outside/Cancel-as-up, PointerUp
		return 0, true
	case 2: // Drag
		return 2, true
	default:
		return 0, false
	}
}
