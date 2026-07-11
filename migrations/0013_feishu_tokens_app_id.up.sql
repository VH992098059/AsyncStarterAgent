-- 飞书的 open_id 按应用隔离：同一人在不同自建应用下 open_id 不同。
-- 旧 token 是用已废弃的全局应用凭证换来的，换成用户自建应用后必然失效，
-- 删除旧记录强制用户重新走 OAuth 授权，再加 NOT NULL 列不需要处理历史数据兼容。
DELETE FROM feishu_tokens;
ALTER TABLE feishu_tokens ADD COLUMN app_id TEXT NOT NULL;
