// Copyright © 2021-2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package solver

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/crillab/gophersat/bf"
	"github.com/crillab/gophersat/explain"
	types "github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
)

type Explainer struct{}

func decodeDimacs(vars map[string]string, dimacs string) (string, error) {
	res := ""
	sc := bufio.NewScanner(bytes.NewBufferString(dimacs))
	lines := strings.Split(dimacs, "\n")
	linenum := 1
SCAN:
	for sc.Scan() {

		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "p":
			continue SCAN
		default:
			for i := 0; i < len(fields)-1; i++ {
				v := fields[i]
				negative := false
				if strings.HasPrefix(fields[i], "-") {
					v = strings.TrimLeft(fields[i], "-")
					negative = true
				}
				variable := vars[v]
				if negative {
					res += fmt.Sprintf("!(%s)", variable)
				} else {
					res += variable
				}

				if i != len(fields)-2 {
					res += fmt.Sprintf(" or ")
				}
			}
			if linenum != len(lines)-1 {
				res += fmt.Sprintf(" and \n")
			}
		}
		linenum++
	}
	if err := sc.Err(); err != nil {
		return res, fmt.Errorf("could not parse problem: %v", err)
	}
	return res, nil
}

func parseVars(r io.Reader) (map[string]string, error) {
	sc := bufio.NewScanner(r)
	res := map[string]string{}
	for sc.Scan() {
		line := sc.Text()
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		switch fields[0] {
		case "c":
			data := strings.Split(fields[1], "=")
			res[data[1]] = data[0]

		default:
			continue

		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("could not parse problem: %v", err)
	}
	return res, nil
}

// Solve tries to find the MUS (minimum unsat) formula from the original problem.
// it returns an error with the decoded dimacs
func (*Explainer) Solve(f bf.Formula, s types.PackageSolver) (types.PackagesAssertions, error) {
	buf := bytes.NewBufferString("")
	if err := bf.Dimacs(f, buf); err != nil {
		return nil, errors.Wrap(err, "cannot extract dimacs from formula")
	}

	copy := *buf

	pb, err := explain.ParseCNF(&copy)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse problem")
	}
	pb2, err := pb.MUS()
	if err != nil {
		return nil, errors.Wrap(err, "could not extract subset")
	}

	variables, err := parseVars(buf)
	if err != nil {
		return nil, errors.Wrap(err, "could not parse variables")
	}

	res, err := decodeDimacs(variables, pb2.CNF())
	if err != nil {
		return nil, errors.Wrap(err, "could not parse dimacs")
	}

	return nil, fmt.Errorf("could not satisfy the constraints: \n%s", res)
}
