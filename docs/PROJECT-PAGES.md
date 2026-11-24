# Project search and pagination

The Projects view searches all saved project names on the server, rather than
filtering a truncated list in the browser. Active and archived views are separate.
Pages contain at most 50 rows; use Newer/Older or return to the newest page.
Changing search or archive view resets the page position. Search treats `%` and
`_` literally and matches case-insensitively using PostgreSQL's text collation.

The Overview shows the six newest active projects and the actual total count,
with a link to the full directory. It does not imply those six are the only saved
projects. Saved settings, run/history actions and archive restoration remain
available for projects beyond the earlier 500-row list limit. Newly created
projects return the directory to the first active page with search cleared.

`GET /api/projects/page` accepts `q`, `archived=true|false`, `limit=1..100` and
`cursor`. The default limit is 50. It returns `items`, `total` for the complete
matching view, global `active_total`/`archived_total`, and an optional `next_cursor`.
The cursor binds the search/archive view to a `(created_at,id)` position; a cursor
from a different filter is rejected. Identical timestamps use IDs as a stable
tie-breaker. A newer project inserted while paging appears when returning to the
newest page, without shifting older rows into duplicate pages. Counts and rows
within each request share a bounded, read-only repeatable-read transaction.

Pagination is not a frozen cross-request snapshot: projects can be edited or
archived between requests. Return to the newest page to refresh a changed view.
The legacy `GET /api/projects` array remains compatible and capped at 500 rows
for existing scripts; the application directory and overview use the new endpoint.
The full project definition is operator-private and remains included for editing.
Support summaries are the separate redacted sharing path.

## Türkçe

Projeler ekranındaki arama, yalnızca ilk 500 kaydı süzmek yerine tüm kayıtlı proje
adlarını sunucuda arar. Aktif ve arşivlenmiş projeler ayrı görünür; her sayfada en
fazla 50 kayıt vardır. Arama veya görünüm değişince ilk sayfaya dönülür. Genel
bakış en yeni altı aktif projeyi ve gerçek toplam sayıyı gösterir. Eski projelerin
ayarlarına ve geçmişine sayfalama veya aramayla ulaşılabilir. Paylaşım için proje
listesi yerine özel bilgileri içermeyen destek özetini kullan.
