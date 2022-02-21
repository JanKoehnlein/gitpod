// Copyright (c) 2020 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

package ports

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

const validTCPInput = `  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000:59D8 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 57008615 1 0000000000000000 100 0 0 10 0
   1: 00000000:17C0 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 57020850 1 0000000000000000 100 0 0 10 0
   2: 0100007F:170C 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 57019442 1 0000000000000000 100 0 0 10 0
   3: 0100007F:EB64 0100007F:59D7 01 00000000:00000000 02:00000348 00000000 33333        0 57010758 2 0000000000000000 20 4 1 10 -1
   4: 940C380A:59D8 0302380A:BFFC 01 00000000:00000000 00:00000000 00000000 33333        0 57015718 3 0000000000000000 20 4 29 61 17
`

const validTCP6Input = `  sl  local_address                         remote_address                        st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 00000000000000000000000000000000:59D7 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 57007063 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000000000000:8C3C 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 57022992 1 0000000000000000 100 0 0 10 0
   2: 00000000000000000000000001000000:170C 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 57019446 1 0000000000000000 100 0 0 10 0
   3: 00000000000000000000000000000000:8CF0 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 57018070 1 0000000000000000 100 0 0 10 0
   4: 0000000000000000FFFF0000940C380A:59D7 0000000000000000FFFF00006100840A:E45C 06 00000000:00000000 03:00001002 00000000     0        0 0 3 0000000000000000
   5: 0000000000000000FFFF0000940C380A:59D7 0000000000000000FFFF00006100840A:E38A 06 00000000:00000000 03:00000D46 00000000     0        0 0 3 0000000000000000
   6: 0000000000000000FFFF0000940C380A:59D7 0000000000000000FFFF0000030C380A:DBFE 01 00000000:00000000 02:000005D2 00000000 33333        0 57015690 2 0000000000000000 20 4 0 10 -1
   7: 0000000000000000FFFF0000940C380A:59D7 0000000000000000FFFF00006100840A:E08A 06 00000000:00000000 03:000003E6 00000000     0        0 0 3 0000000000000000
  20: 0000000000000000FFFF00000100007F:59D7 0000000000000000FFFF00000100007F:EB64 01 00000000:00000000 02:000003D2 00000000 33333        0 57014424 2 0000000000000000 20 4 0 10 -1`

func TestObserve(t *testing.T) {
	type Expectation [][]ServedPort
	tests := []struct {
		Name         string
		FileContents []string
		Expectation  Expectation
	}{
		{
			Name: "basic positive",
			FileContents: []string{
				"", "",
				validTCPInput, validTCP6Input,
			},
			Expectation: Expectation{
				{
					{Address: net.IPv4(127, 0, 0, 1), Port: 5900, BoundToLocalhost: true},
					{Address: net.IPv4zero, Port: 6080},
					{Address: net.IPv4zero, Port: 23000},
					{Address: net.IPv6loopback, Port: 5900, BoundToLocalhost: true},
					{Address: net.IPv6zero, Port: 22999},
					{Address: net.IPv6zero, Port: 35900},
					{Address: net.IPv6zero, Port: 36080},
				},
			},
		},
		{
			Name: "the same port bound locally on ip4 and ip6",
			FileContents: []string{
				"", "",
				`
		   0: 00000000:17C0 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21757239 1 0000000000000000 100 0 0 10 0
		   1: 0100007F:170C 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21752303 1 0000000000000000 100 0 0 10 0
		   2: 00000000:59D8 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21752496 1 0000000000000000 100 0 0 10 0`,
				`
		   0: 00000000000000000000000000000000:EA60 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21750982 1 0000000000000000 100 0 0 10 0
		   1: 00000000000000000000000001000000:170C 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21752306 1 0000000000000000 100 0 0 10 0
		   2: 00000000000000000000000000000000:59D7 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21748173 1 0000000000000000 100 0 0 10 0`,
			},
			Expectation: Expectation{
				{
					{Address: net.IPv4(127, 0, 0, 1), Port: 5900, BoundToLocalhost: true},
					{Address: net.IPv4zero, Port: 6080},
					{Address: net.IPv4zero, Port: 23000},
					{Address: net.IPv6loopback, Port: 5900, BoundToLocalhost: true},
					{Address: net.IPv6zero, Port: 22999},
					{Address: net.IPv6zero, Port: 60000},
				},
			},
		},
		{
			Name: "the same port bound locally for ip4 and globally for ip6",
			FileContents: []string{
				"", "",
				`
   0: 00000000:17C0 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21757239 1 0000000000000000 100 0 0 10 0
   1: 0100007F:170C 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21752303 1 0000000000000000 100 0 0 10 0
   2: 00000000:59D8 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21752496 1 0000000000000000 100 0 0 10 0`,
				`
   0: 00000000000000000000000000000000:EA60 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21750982 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000000000000:170C 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21752306 1 0000000000000000 100 0 0 10 0
   2: 00000000000000000000000000000000:59D7 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21748173 1 0000000000000000 100 0 0 10 0`,
			},
			Expectation: Expectation{
				{
					{Address: net.IPv4(127, 0, 0, 1), Port: 5900, BoundToLocalhost: true},
					{Address: net.IPv4zero, Port: 6080},
					{Address: net.IPv4zero, Port: 23000},
					{Address: net.IPv6zero, Port: 5900, BoundToLocalhost: false},
					{Address: net.IPv6zero, Port: 22999},
					{Address: net.IPv6zero, Port: 60000},
				},
			},
		},
		{
			Name: "the same port bound globally for ip4 and locally for ip6",
			FileContents: []string{
				"", "",
				`
   0: 00000000:17C0 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21757239 1 0000000000000000 100 0 0 10 0
   1: 00000000:170C 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21752303 1 0000000000000000 100 0 0 10 0
   2: 00000000:59D8 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21752496 1 0000000000000000 100 0 0 10 0`,
				`
   0: 00000000000000000000000000000000:EA60 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21750982 1 0000000000000000 100 0 0 10 0
   1: 00000000000000000000000001000000:170C 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21752306 1 0000000000000000 100 0 0 10 0
   2: 00000000000000000000000000000000:59D7 00000000000000000000000000000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 21748173 1 0000000000000000 100 0 0 10 0`,
			},
			Expectation: Expectation{
				{
					{Address: net.IPv4zero, Port: 5900},
					{Address: net.IPv4zero, Port: 6080},
					{Address: net.IPv4zero, Port: 23000},
					{Address: net.IPv6loopback, Port: 5900, BoundToLocalhost: true},
					{Address: net.IPv6zero, Port: 22999},
					{Address: net.IPv6zero, Port: 60000},
				},
			},
		},
		{
			Name: "multiple ports bound locally and globally",
			FileContents: []string{
				"", "",
				`
   0: AD0E600A:240D 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 53934502 1 0000000000000000 100 0 0 10 0
   1: 0100007F:240D 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 53938101 1 0000000000000000 100 0 0 10 0
   2: 00000000:59D8 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 53939555 1 0000000000000000 100 0 0 10 0
   3: 0100007F:6989 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 54384751 1 0000000000000000 100 0 0 10 1024
   4: AD0E600A:6989 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 53934503 1 0000000000000000 100 0 0 10 0
   5: 0100007F:AC48 0100007F:59D7 01 00000000:00000000 02:000004A8 00000000     0        0 53921525 2 0000000000000000 20 4 0 10 -1
   6: 0100007F:6989 0100007F:C74E 01 00000000:00000000 02:0000653F 00000000 33333        0 54366989 2 0000000000000000 20 4 31 10 -1
   7: 0100007F:AC40 0100007F:59D7 01 00000000:00000000 02:000004A8 00000000     0        0 53944840 2 0000000000000000 20 4 0 10 -1
   8: 0100007F:B5D8 0100007F:59D7 01 00000000:00000000 02:0000018F 00000000     0        0 54068010 2 0000000000000000 20 4 0 10 -1
   9: 0100007F:AC42 0100007F:59D7 01 00000000:00000000 02:000004C2 00000000     0        0 53938397 2 0000000000000000 20 4 0 10 -1
   10: 0100007F:AC46 0100007F:59D7 01 00000000:00000000 02:000004C2 00000000     0        0 53921524 2 0000000000000000 20 4 0 10 -1
   11: AD0E600A:B8CC F61BC768:01BB 01 00000000:00000000 02:0000034A 00000000     0        0 53880829 2 0000000000000000 21 4 26 10 -1
   12: 0100007F:C74E 0100007F:6989 01 00000000:00000000 02:0000077F 00000000 33333        0 54384853 2 0000000000000000 20 4 30 10 -1
   13: 0100007F:AC44 0100007F:59D7 01 00000000:00000000 02:00000300 00000000     0        0 53932419 2 0000000000000000 20 4 22 10 -1
   14: 0100007F:AC4A 0100007F:59D7 01 00000000:00000000 02:000004C2 00000000     0        0 53920639 2 0000000000000000 20 4 0 10 -1
   15: AD0E600A:BAEA F61BC768:01BB 01 00000000:00000000 00:00000000 00000000 33333        0 53962195 1 0000000000000000 20 4 28 10 -1
   16: AD0E600A:59D8 760B600A:DA78 01 00000000:00000000 00:00000000 00000000 33333        0 53934581 3 0000000000000000 20 4 27 10 33
   17: 0100007F:C6A6 0100007F:6989 01 00000000:00000000 02:00000360 00000000 33333        0 54383783 2 0000000000000000 20 4 30 10 -1
   18: AD0E600A:59D8 760B600A:DAD0 01 00000000:00000000 00:00000000 00000000 33333        0 53961912 3 0000000000000000 20 4 31 10 -1
   19: 0100007F:AC66 0100007F:59D7 01 00000000:00000000 00:00000000 00000000 33333        0 53939557 1 0000000000000000 20 4 0 10 -1
   20: 0100007F:AE52 0100007F:59D7 01 00000000:00000000 00:00000000 00000000 33333        0 53963729 1 0000000000000000 20 4 0 10 -1
   21: 0100007F:6989 0100007F:C6A6 01 00000000:00000000 02:00006120 00000000 33333        0 54366964 2 0000000000000000 20 4 20 10 -1`,
			},
			Expectation: Expectation{
				{
					{Address: net.IPv4(10, 96, 14, 173), Port: 9229},
					{Address: net.IPv4(127, 0, 0, 1), Port: 9229, BoundToLocalhost: true},
					{Address: net.IPv4zero, Port: 23000},
					{Address: net.IPv4(10, 96, 14, 173), Port: 27017, BoundToLocalhost: false},
					{Address: net.IPv4(127, 0, 0, 1), Port: 27017, BoundToLocalhost: true},
				},
			},
		},
		{
			Name: "multiple ports bound locally and globally",
			FileContents: []string{
				"", "",
				`
   sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode
   0: 220E600A:6989 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 61354170 1 0000000000000000 100 0 0 10 0
   1: 220E600A:240D 00000000:0000 0A 00000000:00000000 00:00000000 00000000     0        0 61354169 1 0000000000000000 100 0 0 10 0
   2: 0100007F:240D 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 61232087 1 0000000000000000 100 0 0 10 0
   3: 00000000:59D8 00000000:0000 0A 00000000:00000000 00:00000000 00000000 33333        0 61285963 1 0000000000000000 100 0 0 10 0
   4: 0100007F:DD60 0100007F:59D7 01 00000000:00000000 02:00000366 00000000     0        0 61278087 2 0000000000000000 20 4 0 10 -1
   5: 0100007F:DD5E 0100007F:59D7 01 00000000:00000000 02:00000380 00000000     0        0 61286915 2 0000000000000000 20 4 0 10 -1
   6: 0100007F:DD64 0100007F:59D7 01 00000000:00000000 02:0000019A 00000000     0        0 61281217 2 0000000000000000 20 4 0 10 -1
   7: 0100007F:E08C 0100007F:59D7 01 00000000:00000000 02:00000233 00000000     0        0 61342352 2 0000000000000000 20 4 0 10 -1
   8: 0100007F:DD82 0100007F:59D7 01 00000000:00000000 00:00000000 00000000 33333        0 61285965 1 0000000000000000 20 4 0 10 -1
   9: 220E600A:59D8 760B600A:E920 01 00000000:00000000 00:00000000 00000000 33333        0 61352088 3 0000000000000000 20 4 27 10 28
   10: 0100007F:DD62 0100007F:59D7 01 00000000:00000000 02:0000019A 00000000     0        0 61278088 2 0000000000000000 20 4 0 10 -1
   11: 0100007F:DD5C 0100007F:59D7 01 00000000:00000000 02:000000FE 00000000     0        0 61232037 2 0000000000000000 20 4 22 10 -1
   12: 220E600A:BE20 F61BC768:01BB 01 00000000:00000000 00:00000000 00000000 33333        0 61342422 1 0000000000000000 21 4 26 10 -1
   13: 0100007F:DD5A 0100007F:59D7 01 00000000:00000000 02:0000019A 00000000     0        0 61232036 2 0000000000000000 20 4 0 10 -1
   14: 0100007F:E0C6 0100007F:59D7 01 00000000:00000000 00:00000000 00000000 33333        0 61342375 1 0000000000000000 20 4 0 10 -1
   15: 220E600A:BAB0 F61BC768:01BB 01 00000000:00000000 02:00000418 00000000     0        0 61278493 2 0000000000000000 20 4 30 10 -1
   16: 220E600A:59D8 760B600A:E96C 01 00000000:00000000 00:00000000 00000000 33333        0 61331984 3 0000000000000000 20 4 31 10 -1`,
			},
			Expectation: Expectation{
				{
					{Address: net.IPv4(10, 96, 14, 34), Port: 9229},
					{Address: net.IPv4(127, 0, 0, 1), Port: 9229, BoundToLocalhost: true},
					{Address: net.IPv4zero, Port: 23000},
					{Address: net.IPv4(10, 96, 14, 34), Port: 27017},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var f int
			obs := PollingServedPortsObserver{
				RefreshInterval: 100 * time.Millisecond,
				fileOpener: func(fn string) (io.ReadCloser, error) {
					if f >= len(test.FileContents) {
						return nil, os.ErrNotExist
					}

					res := io.NopCloser(bytes.NewReader([]byte(test.FileContents[f])))
					f++
					return res, nil
				},
			}

			ctx, cancel := context.WithCancel(context.Background())
			updates, errs := obs.Observe(ctx)
			go func() {
				time.Sleep(500 * time.Millisecond)
				cancel()
			}()
			go func() {
				for range errs {
				}
			}()

			var act Expectation
			for up := range updates {
				act = append(act, up)
			}

			if diff := cmp.Diff(test.Expectation, act); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}

func TestReadNetTCPFile(t *testing.T) {
	type Expectation struct {
		Ports []ServedPort
		Error error
	}
	tests := []struct {
		Name          string
		Input         string
		ListeningOnly bool
		Expectation   Expectation
	}{
		{
			Name:          "valid tcp4 input",
			Input:         validTCPInput,
			ListeningOnly: true,
			Expectation: Expectation{
				Ports: []ServedPort{
					{Address: net.IPv4(127, 0, 0, 1), Port: 5900, BoundToLocalhost: true},
					{Address: net.IPv4zero, Port: 6080},
					{Address: net.IPv4zero, Port: 23000},
				},
			},
		},
		{
			Name:          "valid tcp6 input",
			Input:         validTCP6Input,
			ListeningOnly: true,
			Expectation: Expectation{
				Ports: []ServedPort{
					{Address: net.IPv6loopback, Port: 5900, BoundToLocalhost: true},
					{Address: net.IPv6zero, Port: 22999},
					{Address: net.IPv6zero, Port: 35900},
					{Address: net.IPv6zero, Port: 36080},
				},
			},
		},
		{
			Name:          "invalid input",
			Input:         strings.ReplaceAll(validTCPInput, "0A", ""),
			ListeningOnly: true,
			Expectation:   Expectation{},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			var act Expectation
			act.Ports, act.Error = readNetTCPFile(bytes.NewReader([]byte(test.Input)), test.ListeningOnly)

			if diff := cmp.Diff(test.Expectation, act); diff != "" {
				t.Errorf("unexpected result (-want +got):\n%s", diff)
			}
		})
	}
}
