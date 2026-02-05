# Backend review: payment receipt upload & download

## Current flow

| Action | Route | Handler | Storage |
|--------|--------|---------|---------|
| Upload | `POST /api/v1/payments/:payment_id/upload_receipt` | `UploadReceipt` | `storage.Upload(..., "receipts")` → `receipts/YYYY/MM/<id>.<ext>` |
| Download | `GET /api/v1/payments/:payment_id/download_receipt` | `DownloadReceipt` | `c.File(storage.GetFullPath(payment.DocumentPath))` |

- **Config:** `STORAGE_PATH` (default `./storage`). Base path for all files.
- **DB:** `payments.document_path` stores relative path only.
- **Auth:** Both routes under `protected` (JWT). Download also accepts `?token=...` for img/iframe.

---

## What works well

1. **Download auth** – Only admin, seller, or contract applicant can download. `FindByID` preloads `Contract` so `ApplicantUserID` is available.
2. **Content-type** – Upload validates MIME (pdf, jpeg, jpg, png) via `storage.IsValidContentType`.
3. **Unique filenames** – `generateID()` + extension avoids collisions.
4. **Organized paths** – `receipts/YYYY/MM/` keeps storage tidy.
5. **Contract-scoped upload route** – `UploadReceiptByContract` delegates to same handler; route matches frontend.

---

## Issues and recommendations

### 1. Upload: no authorization (security)

**Issue:** Any authenticated user can call `POST .../upload_receipt` for any `payment_id`. A user could overwrite another applicant’s receipt.

**Recommendation:** Restrict upload to:

- **Applicant** – contract owner for that payment, or  
- **Admin / seller** – can upload on behalf (e.g. approval modal).

**Action:** In `UploadReceipt`, after resolving the payment (and contract), allow only if `currentUserID == payment.Contract.ApplicantUserID` or role is admin/seller.

---

### 2. Upload: no file size limit

**Issue:** `storage.MaxFileSize()` (10 MB) exists but is never used. Large uploads can fill disk or cause DoS.

**Recommendation:** Enforce max size in the handler, e.g.:

- Use `c.Request.ContentLength` and reject if `> MaxFileSize()`, or  
- Use `io.LimitReader(file, MaxFileSize())` when copying (and reject if more bytes remain).

**Action:** Add size check (and/or limit reader) in `UploadReceipt` before calling `storage.Upload`.

---

### 3. Download: path traversal risk

**Issue:** `DocumentPath` comes from the DB. If an attacker or bug wrote a value like `../../../etc/passwd`, `GetFullPath` could point outside the storage root.

**Recommendation:** Resolve the path and ensure it stays under `basePath`:

- `filepath.Clean(relativePath)`  
- Join with base: `filepath.Join(s.basePath, cleanPath)`  
- Ensure result has `filepath.Clean(s.basePath)` as prefix (or use `filepath.Rel` and check no `..`).

**Action:** In storage, add a safe “resolve full path” helper that returns an error if the result is outside base, and use it in download (and anywhere else that serves files by path).

---

### 4. Upload: old file not deleted (cleanup)

**Issue:** When a new receipt is uploaded, `UpdateReceiptPath` overwrites `document_path`. The previous file on disk is never deleted → orphaned files and growing disk usage.

**Recommendation:** Before or after updating the path, if the payment already had a `DocumentPath`, call `storage.Delete(oldPath)` (ignore errors for “file not found”).

**Action:** In `UpdateReceiptPath` (or in the handler before calling it), delete the old file when it exists.

---

### 5. Content-Type vs real content

**Issue:** Only the `Content-Type` header is checked. The file content could be something else (e.g. script). Risk is limited if files are only ever served as download/preview and not executed.

**Recommendation:** For higher assurance, optionally validate magic bytes (PDF, JPEG, PNG) and reject mismatches. Lower priority than 1–3.

---

## Summary

| # | Item | Severity | Suggested action |
|---|------|----------|------------------|
| 1 | Upload authorization | High | Restrict to applicant or admin/seller |
| 2 | File size limit | Medium | Enforce `MaxFileSize()` in upload handler |
| 3 | Path traversal on download | Medium | Validate resolved path under `basePath` |
| 4 | Old file cleanup on replace | Low | Delete previous file when updating receipt |
| 5 | Magic-byte validation | Low | Optional later improvement |

**Implemented (in code):** Items 1–3 are implemented: upload restricted to applicant or admin/seller, file size check (ContentLength), and `storage.SafeFullPath` used for download to prevent path traversal.
