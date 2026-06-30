# Local Toolchain

This repository expects the local Go toolchain at:

`C:\Users\1003584\.g\versions\1.26.1\bin\go.exe`

Use from PowerShell:

```powershell
$env:PATH = "C:\Users\1003584\.g\versions\1.26.1\bin;$env:PATH"
go version
go test ./cmd/... ./internal/...
```

Do not run `go test ./...` because local tool payloads may exist under `.tools`.
Prefer:

```powershell
go test ./cmd/... ./internal/...
```
