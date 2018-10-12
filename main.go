package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)

const (
	FlagSource = "source"
	FlagStore  = "store"
	FlagImages = "images"
)

type AutoWall struct {
	dir    string
	source string
	num    int
	mu     sync.Mutex
}

func main() {
	Execute(os.Args[1:])
}

func Execute(args []string) {
	RootCmd.SetArgs(args)
	Must(RootCmd.Execute())
}

func init() {
	RootCmd.PersistentFlags().StringP(FlagSource, "s", "https://www.wallpaperup.com/wallpaper/download", "source of wallpapers")
	RootCmd.PersistentFlags().StringP(FlagStore, "f", "~/wallpapers", "directory to store images")
	RootCmd.PersistentFlags().IntP(FlagImages, "n", 30, "number of images to download")
}

func Must(err error) {
	if err != nil {
		fmt.Print(err)
		os.Exit(1)
	}
}

var RootCmd = &cobra.Command{
	Use:   "autowall",
	Short: "download several wallpapers from a source",
	Run: func(cmd *cobra.Command, args []string) {
		source, err := cmd.PersistentFlags().GetString(FlagSource)
		Must(err)
		d, err := cmd.PersistentFlags().GetString(FlagStore)
		Must(err)
		num, err := cmd.PersistentFlags().GetInt(FlagImages)
		Must(err)

		dir, err := homedir.Expand(d)
		Must(err)

		a := &AutoWall{
			dir:    dir,
			source: source,
			num:    num,
		}

		Must(a.clearDir())
		Must(a.getImages())
	},
}

func (a *AutoWall) getImages() error {
	var nums []int
	rand.Seed(time.Now().UTC().UnixNano())
	for i := 0; i < a.num; i++ {
		nums = append(nums, (rand.Int() % 130000))
	}

	var wg sync.WaitGroup
	var result *multierror.Error
	wg.Add(len(nums))

	downloadFunc := func(source, target int) {
		defer wg.Done()

		var err error
		body := []byte("<!DOCTYPE html>")
		res := new(http.Response)
		for res.StatusCode != 200 && !bytes.Contains(body, []byte("DOCTYPE html")) {
			res, err = http.Get(a.buildSourcePath(source))
			if err != nil {
				a.mu.Lock()
				result = multierror.Append(result, err)
				a.mu.Unlock()
				return
			}
			defer res.Body.Close()

			body, err = ioutil.ReadAll(res.Body)
			if err != nil {
				a.mu.Lock()
				result = multierror.Append(result, err)
				a.mu.Unlock()
				return
			}

			source++
		}

		f, err := os.Create(a.buildFilePath(target))
		if err != nil {
			a.mu.Lock()
			result = multierror.Append(result, err)
			a.mu.Unlock()
			return
		}

		if _, err := f.Write(body); err != nil {
			a.mu.Lock()
			result = multierror.Append(result, err)
			a.mu.Unlock()
			return
		}
		f.Close()
	}

	for i, n := range nums {
		go downloadFunc(n, i)
	}

	wg.Wait()

	return result.ErrorOrNil()
}

func (a *AutoWall) buildSourcePath(n int) string {
	return fmt.Sprintf("%s/%s", a.source, strconv.Itoa(n))
}

func (a *AutoWall) buildFilePath(n int) string {
	return filepath.Join(a.dir, strconv.Itoa(n)+".jpg")
}

func (a *AutoWall) clearDir() error {
	info, err := os.Stat(a.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return os.Mkdir(a.dir, os.ModePerm)
		}
		return err
	}

	if !info.IsDir() {
		return fmt.Errorf("path is not directory: %s", a.dir)
	}

	d, err := os.Open(a.dir)
	if err != nil {
		return err
	}
	defer d.Close()

	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}

	var result *multierror.Error
	for _, name := range names {
		err = os.RemoveAll(filepath.Join(a.dir, name))
		if err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result.ErrorOrNil()
}
