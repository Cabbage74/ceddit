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

DROP TABLE IF EXISTS `outbox`;
DROP TABLE IF EXISTS `follower`;
DROP TABLE IF EXISTS `following`;

CREATE TABLE `following` (
    `id` BIGINT UNSIGNED NOT NULL COMMENT 'Snowflake ID, 关系聚合根',
    `from_user_id` BIGINT UNSIGNED NOT NULL COMMENT '关注者用户ID',
    `to_user_id` BIGINT UNSIGNED NOT NULL COMMENT '被关注者用户ID',
    `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_from_to` (`from_user_id`, `to_user_id`),
    KEY `idx_from_created` (`from_user_id`, `created_at`, `to_user_id`),
    KEY `idx_to` (`to_user_id`, `from_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `follower` (
    `id` BIGINT UNSIGNED NOT NULL COMMENT '投影行ID (使用事件 aggregate_id)',
    `to_user_id` BIGINT UNSIGNED NOT NULL COMMENT '被关注者用户ID (粉丝列表查询维度)',
    `from_user_id` BIGINT UNSIGNED NOT NULL COMMENT '粉丝用户ID',
    `created_at` DATETIME(3) NOT NULL COMMENT '关注时间 (来自事件 occurred_at)',
    `updated_at` DATETIME(3) NOT NULL COMMENT '最后更新时间',
    `last_event_id` VARCHAR(128) DEFAULT NULL COMMENT '最近处理的事件ID，用于消费幂等',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_to_from` (`to_user_id`, `from_user_id`),
    KEY `idx_to_created` (`to_user_id`, `created_at`, `from_user_id`),
    KEY `idx_from` (`from_user_id`, `to_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

CREATE TABLE `outbox` (
    `id` BIGINT UNSIGNED NOT NULL COMMENT 'Snowflake ID',
    `aggregate_type` VARCHAR(64) NOT NULL COMMENT '聚合类型: FOLLOW / UNFOLLOW',
    `aggregate_id` BIGINT UNSIGNED NOT NULL COMMENT 'following.id',
    `type` VARCHAR(64) NOT NULL DEFAULT 'USER_RELATION_CHANGE' COMMENT '事件类型',
    `payload` JSON NOT NULL COMMENT '事件载荷',
    `created_at` TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    PRIMARY KEY (`id`),
    KEY `ix_outbox_agg` (`aggregate_type`, `aggregate_id`),
    KEY `ix_outbox_ct` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
