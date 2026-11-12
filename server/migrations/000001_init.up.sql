-- 000001_init: 基线结构(与 GORM 模型 AutoMigrate 输出等价)。
-- CREATE TABLE IF NOT EXISTS:兼容"先有库表、后引入迁移"的存量环境,首次执行记为版本 1。
-- 后续结构演进请追加 000002+ 迁移文件,不要修改本文件。

CREATE TABLE IF NOT EXISTS `users` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `username`      VARCHAR(64)  NOT NULL,
    `password_hash` VARCHAR(128) NOT NULL,
    `nickname`      VARCHAR(64)  NULL,
    `role`          VARCHAR(16)  NOT NULL DEFAULT 'user',
    `status`        TINYINT      NOT NULL DEFAULT '1',
    `created_at`    DATETIME(3)  NULL,
    `updated_at`    DATETIME(3)  NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uni_users_username` (`username`),
    KEY `idx_users_role` (`role`),
    KEY `idx_users_status` (`status`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `categories` (
    `id`   BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `name` VARCHAR(32) NOT NULL,
    `sort` INT         NOT NULL DEFAULT '0',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uni_categories_name` (`name`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `products` (
    `id`          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `category_id` BIGINT UNSIGNED NOT NULL,
    `name`        VARCHAR(128) NOT NULL,
    `age_group`   VARCHAR(16)  NOT NULL,
    `material`    VARCHAR(64)  NOT NULL,
    `season`      VARCHAR(16)  NOT NULL,
    `description` TEXT         NULL,
    `image`       VARCHAR(512) NULL,
    `price`       BIGINT       NOT NULL,
    `status`      TINYINT      NOT NULL DEFAULT '1',
    `recommended` TINYINT      NOT NULL DEFAULT '0',
    `created_at`  DATETIME(3)  NULL,
    `updated_at`  DATETIME(3)  NULL,
    PRIMARY KEY (`id`),
    KEY `idx_products_category_id` (`category_id`),
    KEY `idx_products_age_group` (`age_group`),
    KEY `idx_products_status` (`status`),
    KEY `idx_products_recommended` (`recommended`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `skus` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `product_id` BIGINT UNSIGNED NOT NULL,
    `size`       VARCHAR(16) NOT NULL,
    `stock`      INT         NOT NULL DEFAULT '0',
    PRIMARY KEY (`id`),
    UNIQUE KEY `uk_product_size` (`product_id`, `size`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `orders` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `order_no`      VARCHAR(32)  NOT NULL,
    `user_id`       BIGINT UNSIGNED NOT NULL,
    `status`        VARCHAR(24)  NOT NULL,
    `total_amount`  BIGINT       NOT NULL,
    `pay_amount`    BIGINT       NOT NULL,
    `receiver`      VARCHAR(64)  NOT NULL,
    `phone`         VARCHAR(32)  NOT NULL,
    `address`       VARCHAR(255) NOT NULL,
    `remark`        VARCHAR(255) NULL,
    `pay_time`      DATETIME(3)  NULL,
    `ship_time`     DATETIME(3)  NULL,
    `finish_time`   DATETIME(3)  NULL,
    `cancel_time`   DATETIME(3)  NULL,
    `cancel_reason` VARCHAR(128) NULL,
    `created_at`    DATETIME(3)  NULL,
    `updated_at`    DATETIME(3)  NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uni_orders_order_no` (`order_no`),
    KEY `idx_orders_user_id` (`user_id`),
    KEY `idx_orders_status` (`status`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `order_items` (
    `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `order_id`     BIGINT UNSIGNED NOT NULL,
    `product_id`   BIGINT UNSIGNED NOT NULL,
    `product_name` VARCHAR(128) NOT NULL,
    `image`        VARCHAR(512) NULL,
    `size`         VARCHAR(16)  NOT NULL,
    `price`        BIGINT       NOT NULL,
    `quantity`     INT          NOT NULL,
    PRIMARY KEY (`id`),
    KEY `idx_order` (`order_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;

CREATE TABLE IF NOT EXISTS `addresses` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `user_id`    BIGINT UNSIGNED NOT NULL,
    `receiver`   VARCHAR(32)  NOT NULL,
    `phone`      VARCHAR(32)  NOT NULL,
    `province`   VARCHAR(32)  NOT NULL,
    `city`       VARCHAR(32)  NOT NULL,
    `district`   VARCHAR(32)  NOT NULL,
    `detail`     VARCHAR(255) NOT NULL,
    `is_default` TINYINT(1)   NOT NULL DEFAULT '0',
    `created_at` DATETIME(3)  NULL,
    `updated_at` DATETIME(3)  NULL,
    PRIMARY KEY (`id`),
    KEY `idx_addresses_user_id` (`user_id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;
