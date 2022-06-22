// Copyright © 2021 Ettore Di Giacinto <mudler@mocaccino.org>
//
// This program is free software; you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation; either version 2 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License along
// with this program; if not, see <http://www.gnu.org/licenses/>.

package logger

import (
	"errors"
	"os"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

func IsTerminal() bool {
	return isatty.IsTerminal(os.Stdout.Fd())
}

// GetTerminalSize returns the width and the height of the active terminal.
func GetTerminalSize() (width, height int, err error) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if w <= 0 {
		w = 0
	}
	if h <= 0 {
		h = 0
	}
	if err != nil {
		err = errors.New("size not detectable")
	}
	return w, h, err
}
