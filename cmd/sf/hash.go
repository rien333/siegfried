// Copyright 2015 Richard Lehane. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"hash"
	"hash/crc32"
)

const hashChoices = "'md5', 'sha1', 'sha256', 'sha512', 'crc'"

func getHash(typ string) hash.Hash {
	switch typ {
	case "":
	case "md5", "MD5":
		return md5.New()
	case "sha1", "SHA1":
		return sha1.New()
	case "sha256", "SHA256":
		return sha256.New()
	case "sha512", "SHA512":
		return sha512.New()
	case "crc", "CRC":
		return crc32.NewIEEE()
	}
	return nil
}

func hashHeader(pad bool, typ string) string {
	switch typ {
	default:
		return "no"
	case "md5", "MD5":
		if pad {
			return "md5   "
		}
		return "md5"
	case "sha1", "SHA1":
		if pad {
			return "sha1  "
		}
		return "sha1"
	case "sha256", "SHA256":
		if pad {
			return "sha256"
		}
		return "sha256"
	case "sha512", "SHA512":
		if pad {
			return "sha512"
		}
		return "sha512"
	case "crc", "CRC":
		if pad {
			return "crc   "
		}
		return "crc"
	}
}
