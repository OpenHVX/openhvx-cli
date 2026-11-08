package cli

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"strings"

	"hvx-cli/internal/api"
)

func (a *App) getImages(args []string) error {
	if err := a.ensureAccess("get images", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("get images", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() != 0 {
		return ErrUsage
	}

	cfg := a.cfg
	var (
		list    func() ([]api.Image, error)
		listRaw func() ([]byte, error)
	)

	switch cfg.Mode() {
	case "tenant":
		client := api.NewClient(cfg.TenantServer(), cfg.Token(), cfg.Auth.Insecure)
		list = client.ListTenantImages
		listRaw = client.ListTenantImagesRaw
	case "admin":
		client := api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
		list = client.ListAdminImages
		listRaw = client.ListAdminImagesRaw
	default:
		return fmt.Errorf("invalid mode '%s'", cfg.Mode())
	}

	if a.outputFormat == "json" {
		raw, err := listRaw()
		if err != nil {
			return err
		}
		_, err = a.stdout.Write(raw)
		return err
	}

	images, err := list()
	if err != nil {
		return err
	}
	if len(images) == 0 {
		fmt.Fprintln(a.stdout, "No images found.")
		return nil
	}

	columns := []tableColumn{
		{Header: "ID", Width: 36},
		{Header: "SCOPE", Width: 10},
		{Header: "OS", Width: 8},
		{Header: "ARCH", Width: 6},
		{Header: "GEN", Width: 4},
	}
	rows := make([][]string, 0, len(images))
	for _, img := range images {
		rows = append(rows, []string{
			truncate(img.ID, 36),
			truncate(firstNonEmpty(img.Scope, img.Status), 10),
			truncate(img.OS, 8),
			truncate(img.Arch, 6),
			fmt.Sprintf("%d", img.Gen),
		})
	}
	a.printTable(columns, rows, " ")
	return nil
}

func (a *App) describeImage(args []string) error {
	if err := a.ensureAccess("describe image", modeTenant|modeAdmin); err != nil {
		return err
	}

	fs := flag.NewFlagSet("describe image", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	resolveFlag := fs.Bool("resolve", false, "resolve the image reference (raw response)")
	if err := parseFlags(fs, args); err != nil {
		return ErrUsage
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("'hvx describe image' requires an IMAGE_ID")
	}
	if fs.NArg() > 1 {
		return ErrUsage
	}
	imageID := strings.TrimSpace(fs.Arg(0))
	if imageID == "" {
		return fmt.Errorf("IMAGE_ID cannot be empty")
	}

	cfg := a.cfg
	var (
		get     func(string) (*api.Image, error)
		getRaw  func(string) ([]byte, error)
		resolve func(string) ([]byte, error)
	)

	switch cfg.Mode() {
	case "tenant":
		client := api.NewClient(cfg.TenantServer(), cfg.Token(), cfg.Auth.Insecure)
		get = client.GetTenantImage
		getRaw = client.GetTenantImageRaw
		resolve = client.ResolveTenantImage
	case "admin":
		client := api.NewClient(cfg.AdminServer(), cfg.Token(), cfg.Auth.Insecure)
		get = client.GetAdminImage
		getRaw = client.GetAdminImageRaw
		resolve = client.ResolveAdminImage
	default:
		return fmt.Errorf("invalid mode '%s'", cfg.Mode())
	}

	if *resolveFlag {
		raw, err := resolve(imageID)
		if err != nil {
			return err
		}
		raw = append(bytes.TrimSpace(raw), '\n')
		_, err = a.stdout.Write(raw)
		return err
	}

	if a.outputFormat == "json" {
		raw, err := getRaw(imageID)
		if err != nil {
			return err
		}
		raw = append(bytes.TrimSpace(raw), '\n')
		_, err = a.stdout.Write(raw)
		return err
	}

	img, err := get(imageID)
	if err != nil {
		return err
	}
	fmt.Fprintf(a.stdout, "ID: %s\n", img.ID)
	fmt.Fprintf(a.stdout, "Name: %s\n", img.Name)
	fmt.Fprintf(a.stdout, "Scope: %s\n", firstNonEmpty(img.Scope, img.Status))
	fmt.Fprintf(a.stdout, "OS/Arch: %s/%s\n", img.OS, img.Arch)
	fmt.Fprintf(a.stdout, "Generation: %d\n", img.Gen)
	if img.SizeBytes > 0 {
		fmt.Fprintf(a.stdout, "Size: %d bytes\n", img.SizeBytes)
	}
	if img.MTime != "" {
		fmt.Fprintf(a.stdout, "Updated: %s\n", img.MTime)
	}
	if strings.TrimSpace(img.Path) != "" {
		fmt.Fprintf(a.stdout, "Path: %s\n", img.Path)
	}
	if img.Description != "" {
		fmt.Fprintf(a.stdout, "Description: %s\n", img.Description)
	}
	return nil
}
