-- 000002_finance: 财务中心数据层。
-- pay_flows        资金流水(insert-only 账本:退款只追加出账流水,不修改/不冲销原支付流水)
-- refund_orders    退款售后单(管理员直接退款 / 用户申请待审核)
-- daily_statements 日结账单(每日一条,定时任务/手动触发按 stat_date 幂等 upsert)
-- orders 补 refund_time;存量已支付订单回填进账流水,保证上线即可对账。

CREATE TABLE IF NOT EXISTS `pay_flows` (
    `id`         BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `flow_no`    VARCHAR(32)  NOT NULL,
    `order_no`   VARCHAR(32)  NOT NULL,
    `user_id`    BIGINT UNSIGNED NOT NULL,
    `direction`  TINYINT      NOT NULL,
    `biz_type`   VARCHAR(16)  NOT NULL,
    `amount`     BIGINT       NOT NULL,
    `status`     VARCHAR(16)  NOT NULL DEFAULT 'success',
    `remark`     VARCHAR(255) NULL,
    `created_at` DATETIME(3)  NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uni_pay_flows_flow_no` (`flow_no`),
    KEY `idx_pay_flows_order_no` (`order_no`),
    KEY `idx_pay_flows_created_at` (`created_at`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;

-- direction: 1=进账(支付成功) 2=出账(退款成功);amount 恒为正,单位分
CREATE TABLE IF NOT EXISTS `refund_orders` (
    `id`           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `refund_no`    VARCHAR(32)  NOT NULL,
    `order_no`     VARCHAR(32)  NOT NULL,
    `user_id`      BIGINT UNSIGNED NOT NULL,
    `amount`       BIGINT       NOT NULL,
    `reason`       VARCHAR(255) NULL,
    `status`       VARCHAR(16)  NOT NULL DEFAULT 'pending',
    `operator_id`  BIGINT UNSIGNED NULL,
    `created_at`   DATETIME(3)  NULL,
    `processed_at` DATETIME(3)  NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uni_refund_orders_refund_no` (`refund_no`),
    KEY `idx_refund_orders_order_no` (`order_no`),
    KEY `idx_refund_orders_status` (`status`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;

-- status: pending(待审核) refunded(已退款) rejected(已拒绝)
CREATE TABLE IF NOT EXISTS `daily_statements` (
    `id`            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    `stat_date`     DATE NOT NULL,
    `pay_amount`    BIGINT NOT NULL DEFAULT 0,
    `refund_amount` BIGINT NOT NULL DEFAULT 0,
    `net_amount`    BIGINT NOT NULL DEFAULT 0,
    `pay_count`     INT    NOT NULL DEFAULT 0,
    `refund_count`  INT    NOT NULL DEFAULT 0,
    `created_at`    DATETIME(3) NULL,
    `updated_at`    DATETIME(3) NULL,
    PRIMARY KEY (`id`),
    UNIQUE KEY `uni_daily_statements_stat_date` (`stat_date`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_general_ci;

ALTER TABLE `orders` ADD COLUMN `refund_time` DATETIME(3) NULL AFTER `cancel_reason`;

-- 存量已支付订单回填进账流水:flow_no = 'LS' + 订单号去 'FK' 前缀(订单号唯一,天然不冲突)
INSERT INTO `pay_flows`
    (`flow_no`, `order_no`, `user_id`, `direction`, `biz_type`, `amount`, `status`, `remark`, `created_at`)
SELECT CONCAT('LS', SUBSTRING(o.`order_no`, 3)), o.`order_no`, o.`user_id`, 1, 'pay',
       o.`pay_amount`, 'success', '存量数据回填', o.`pay_time`
FROM `orders` o
WHERE o.`pay_time` IS NOT NULL;
