# Static pages in the shared directory

No separate site directory or URL prefix is required. The existing `RS_ROOT` serves both directory listings and static pages.

- A directory request ending in `/` serves its regular, readable `index.html` when present. Otherwise, the NAIS listing appears.
- Requests without a directory trailing slash redirect before rendering, so relative asset links work.
- Direct `.html` and `.htm` requests render in the browser, including case-insensitive extensions.
- CSS, JS/MJS, JSON/source maps, common images, web fonts and WASM use explicit inline MIME types. Other extensions remain attachment downloads.
- HEAD and byte ranges are supported. PHP, CGI and other server-side code are never executed.

Example layout inside your existing share:

```text
public-storage/
  test.txt
  brochure.html
  demo/
    index.html
    style.css
    app.js
```

`/brochure.html` renders directly. `/demo/` renders its index and can use relative `style.css` and `app.js` references. Placing `index.html` at the share root makes `/` show that page; removing/renaming it restores the directory listing.

Publishing HTML intentionally permits browser-side code under this domain. Static pages can use inline scripts/styles and external assets. Only trusted administrators should place active content in the share. The listing retains its more restrictive content policy. No authentication or server-side execution was added; hidden paths, traversal, escaping symlinks and write methods remain blocked.

Deploy the new binary once using the normal update procedure. Afterwards, changing pages and assets on USB requires no rebuild or service restart; the next request reads the current files. No new env settings, gateway rules or TLS certificates are needed.
