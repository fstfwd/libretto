// Copyright 2014-2015 Apcera Inc. All rights reserved.

// +build windows

package terminal

import "fmt"

// Using basic colors for Windows:
// http://ascii-table.com/ansi-escape-sequences.php
var ColorError int = 35
var ColorWarn int = 33
var ColorSuccess int = 32
var ColorNeutral int = 36
var BackgroundColorBlack = "\033[30;49m"
var BackgroundColorWhite = "\033[30;47m"
var ResetCode string = "\033[0m"

//---------------------------------------------------------------------------
// Display helpers
//---------------------------------------------------------------------------

func Colorize(color int, msg string) string {
	//FIXME(Sha): This logic assumes that because tty is set that the terminal supports colors.
	//	This needs to be fixed via bringing in PDCurses or some other library.
	if !stdoutIsTTY {
		return msg
	}

	// Uses basic color set rather than extended set.
	return fmt.Sprintf("\033[0;%dm%s%s", color, msg, ResetCode)
}

// BoldText returns the bold-ified version of the passed-in string.
func BoldText(msg string) string {
	return msg
}
