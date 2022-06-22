// Copyright © 2022 Ettore Di Giacinto <mudler@mocaccino.org>
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

package database

import (
	storm "github.com/asdine/storm"
	"github.com/mudler/luet/pkg/api/core/types"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

type schemaMigration func(*storm.DB) error

var migrations = []schemaMigration{migrateDefaultPackage}

var migrateDefaultPackage schemaMigration = func(bs *storm.DB) error {
	packs := []types.Package{}

	bs.Bolt.View(
		func(tx *bbolt.Tx) error {
			// previously we had pkg.DefaultPackage
			// IF it's there, collect packages to add to the new schema
			b := tx.Bucket([]byte("DefaultPackage"))
			if b != nil {
				b.ForEach(func(k, v []byte) error {
					p, err := types.PackageFromYaml(v)
					if err == nil && p.ID != 0 {
						packs = append(packs, p)
					}
					return nil
				})
			}
			return nil
		},
	)

	for k := range packs {
		d := &packs[k]
		d.ID = 0
		err := bs.Save(d)
		if err != nil {
			return errors.Wrap(err, "Error saving package to "+d.Path)
		}
	}

	// Be sure to delete old only if everything was migrated without any error
	bs.Bolt.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("DefaultPackage"))
		if b != nil {
			return tx.DeleteBucket([]byte("DefaultPackage"))
		}
		return nil
	})

	return nil
}
