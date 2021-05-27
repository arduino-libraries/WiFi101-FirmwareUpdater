/*
  FirmwareUploader
  Copyright (c) 2021 Arduino LLC.  All right reserved.

  This library is free software; you can redistribute it and/or
  modify it under the terms of the GNU Lesser General Public
  License as published by the Free Software Foundation; either
  version 2.1 of the License, or (at your option) any later version.

  This library is distributed in the hope that it will be useful,
  but WITHOUT ANY WARRANTY; without even the implied warranty of
  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the GNU
  Lesser General Public License for more details.

  You should have received a copy of the GNU Lesser General Public
  License along with this library; if not, write to the Free Software
  Foundation, Inc., 51 Franklin St, Fifth Floor, Boston, MA  02110-1301  USA
*/

package indexes

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"path"

	"github.com/arduino/arduino-cli/arduino/cores/packageindex"
	"github.com/arduino/arduino-cli/arduino/security"
	"github.com/arduino/arduino-cli/arduino/utils"
	"github.com/arduino/go-paths-helper"
	"go.bug.st/downloader/v2"
)

// DownloadIndex will download the index in the os temp directory
func DownloadIndex(indexURL string) error {
	indexpath := paths.New(paths.TempDir().String(), "fwuploader")

	URL, err := utils.URLParse(indexURL)
	if err != nil {
		return fmt.Errorf("unable to parse URL %s: %s", indexURL, err)
	}

	// Download index
	var tmpIndex *paths.Path
	if tmpFile, err := ioutil.TempFile("", ""); err != nil {
		return fmt.Errorf("creating temp file for index download: %s", err)
	} else if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("creating temp file for index download: %s", err)
	} else {
		tmpIndex = paths.New(tmpFile.Name() + ".json")
		// TODO remove tmpFile
	}
	defer tmpIndex.Remove()
	d, err := downloader.Download(tmpIndex.String(), URL.String())
	if err != nil {
		return fmt.Errorf("downloading index %s: %s", indexURL, err)
	}
	indexPath := indexpath.Join(path.Base(URL.Path))
	Download(d)
	if d.Error() != nil {
		return fmt.Errorf("downloading index %s: %s", URL, d.Error())
	}

	// Check for signature
	var tmpSig *paths.Path
	var indexSigPath *paths.Path

	URLSig, err := url.Parse(URL.String())
	if err != nil {
		return fmt.Errorf("parsing url for index signature check: %s", err)
	}
	URLSig.Path += ".sig"

	if t, err := ioutil.TempFile("", ""); err != nil {
		return fmt.Errorf("creating temp file for index signature download: %s", err)
	} else if err := t.Close(); err != nil {
		return fmt.Errorf("creating temp file for index signature download: %s", err)
	} else {
		tmpSig = paths.New(t.Name() + ".sig")
		// TODO remove tmpSig
	}
	defer tmpSig.Remove()
	d, err = downloader.Download(tmpSig.String(), URLSig.String())
	if err != nil {
		return fmt.Errorf("downloading index signature %s: %s", URLSig, err)
	}

	indexSigPath = indexpath.Join(path.Base(URLSig.Path))
	Download(d)
	if d.Error() != nil {
		return fmt.Errorf("downloading index signature %s: %s", URL, d.Error())
	}

	valid, _, err := security.VerifyArduinoDetachedSignature(tmpIndex, tmpSig)
	if err != nil {
		return fmt.Errorf("signature verification error: %s", err)
	}
	if !valid {
		return fmt.Errorf("index has an invalid signature")
	}
	// the signature verification is already done above
	if _, err := packageindex.LoadIndexNoSign(tmpIndex); err != nil {
		return fmt.Errorf("invalid package index in %s: %s", URL, err)
	}

	if err := indexpath.MkdirAll(); err != nil { //does not overwrite
		return fmt.Errorf("can't create data directory %s: %s", indexpath, err)
	}

	if err := tmpIndex.CopyTo(indexPath); err != nil { //does overwrite if already present
		return fmt.Errorf("saving downloaded index %s: %s", URL, err)
	}
	if tmpSig != nil {
		if err := tmpSig.CopyTo(indexSigPath); err != nil { //does overwrite if already present
			return fmt.Errorf("saving downloaded index signature: %s", err)
		}
	}
	return nil
}

func Download(d *downloader.Downloader) error {
	if d == nil {
		// This signal means that the file is already downloaded
		return nil
	}
	err := d.Run()
	if err != nil {
		return fmt.Errorf("failed to download file from %s : %s", d.URL, err)
	}
	// The URL is not reachable for some reason
	if d.Resp.StatusCode >= 400 && d.Resp.StatusCode <= 599 {
		return errors.New(d.Resp.Status)
	}
	return nil
}
