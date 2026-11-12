# HSSH Distribution & APT Repository Guide

This guide details how to publish `hssh` and distribute it across Ubuntu/Debian (`apt`), macOS (`brew`), and Windows.

---

## 1. Automated Release Pipeline (GoReleaser + GitHub Actions)

We have configured [`.goreleaser.yaml`](.goreleaser.yaml) and [`.github/workflows/release.yml`](.github/workflows/release.yml).

### How to trigger a release:
```bash
git add .
git commit -m "feat: release v1.0.0"
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin main --tags
```

### What happens automatically:
1. GitHub Actions detects the `v1.0.0` tag.
2. **GoReleaser** compiles static binaries for:
   - Linux (`amd64`, `arm64`)
   - macOS (`amd64`, Apple Silicon `arm64`)
   - Windows (`amd64` `.exe`)
3. **NFPM** automatically packages native:
   - Debian / Ubuntu: `hssh_1.0.0_linux_amd64.deb` and `hssh_1.0.0_linux_arm64.deb`
   - Fedora / RHEL: `hssh_1.0.0_linux_amd64.rpm`
4. Attaches checksums (`checksums.txt`) and publishes the release assets directly to GitHub Releases.

---

## 2. Distributing to Ubuntu / Debian (`sudo apt install`)

There are three primary methods to distribute `.deb` packages to Ubuntu users:

### Option A: Direct `.deb` Installation (Simplest, Zero Config)
Users on Ubuntu or Debian run:
```bash
curl -LO https://github.com/ilhammmaulana/hssh/releases/latest/download/hssh_linux_amd64.deb
sudo apt install ./hssh_linux_amd64.deb
```
*(Using `apt install ./file.deb` automatically resolves dependencies and installs the binary to `/usr/bin/hssh`).*

---

### Option B: Free Hosted APT Repository via Cloudsmith (Recommended for `sudo apt install hssh`)

[Cloudsmith](https://cloudsmith.com) provides free, automated package repositories for open-source projects.

1. Create a free open-source account on Cloudsmith.
2. Create an open-source repository named `hssh`.
3. Add your `CLOUDSMITH_API_KEY` to GitHub Secrets.
4. Add the Cloudsmith publisher to `.goreleaser.yaml`:
   ```yaml
   publishers:
     - name: cloudsmith
       ids:
         - packages
       cmd: cloudsmith push deb ilhammmaulana/hssh/any-distro/any-version {{ .ArtifactPath }}
   ```
5. Users on Ubuntu can then install cleanly via:
   ```bash
   curl -1sLf 'https://dl.cloudsmith.io/public/ilhammmaulana/hssh/setup.deb.sh' | sudo -E bash
   sudo apt update
   sudo apt install hssh
   ```

---

### Option C: GitHub Pages APT Repository

You can host an APT repository directly on GitHub Pages using `apt-ftparchive` or `reprepro`:

1. Every time a `.deb` is built, a GitHub Action adds it to `god-ssh-docs/apt/`.
2. Generates the `Packages.gz` and `Release` files.
3. Users add your repository URL:
   ```bash
   echo "deb [trusted=yes] https://ilhammmaulana.github.io/hssh/apt stable main" | sudo tee /etc/apt/sources.list.d/hssh.list
   sudo apt update
   sudo apt install hssh
   ```

---

## 3. SEO & Website Verification Checklist

The documentation site at `god-ssh-docs/index.html` has been optimized with:
- **Canonical tag**: Points to `https://ilhammmaulana.github.io/hssh/`
- **Open Graph & Twitter Cards**: High-resolution preview image embed for social shares.
- **JSON-LD Schema**: Structured `SoftwareApplication` data for Google Rich Results.
- **Sitemap**: [`sitemap.xml`](sitemap.xml) for search engine indexing.
- **Robots.txt**: [`robots.txt`](robots.txt) allowing full crawler access.
