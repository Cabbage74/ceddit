DROP TABLE IF EXISTS `post`;
DROP TABLE IF EXISTS `user`;

CREATE TABLE `user` (
    `id` bigint(20) NOT NULL AUTO_INCREMENT,
    `user_id` bigint(20) NOT NULL,
    `username` varchar(64) COLLATE utf8mb4_general_ci NOT NULL,
    `password` varchar(64) COLLATE utf8mb4_general_ci NOT NULL,
    `email` varchar(64) COLLATE utf8mb4_general_ci,
    `gender` tinyint(4) NOT NULL DEFAULT '0',
    `create_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP,
    `update_time` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    UNIQUE KEY `idx_username` (`username`) USING BTREE,
    UNIQUE KEY `idx_user_id` (`user_id`) USING BTREE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

CREATE TABLE `post` (
    `id` bigint(20) NOT NULL COMMENT '雪花ID，主键',
    `title` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '标题',
    `description` varchar(256) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'AI摘要',
    `content_object_key` varchar(512) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'COS对象Key',
    `content_etag` varchar(128) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT 'COS ETag',
    `content_size` bigint(20) unsigned DEFAULT NULL COMMENT '正文大小(字节)',
    `content_sha256` char(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '正文SHA-256',
    `author_id` bigint(20) NOT NULL COMMENT '作者用户ID',
    `status` varchar(16) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'draft' COMMENT 'draft | published',
    `create_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `publish_time` timestamp NULL DEFAULT NULL COMMENT '正式发布时间',
    `update_time` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`),
    KEY `ix_post_author_ct` (`author_id`, `create_time`),
    KEY `ix_post_status_ct` (`status`, `create_time`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
