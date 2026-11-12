-- 000002_finance down:回滚财务中心(流水/售后单/日结数据一并删除,不可恢复)。
DROP TABLE IF EXISTS `daily_statements`;
DROP TABLE IF EXISTS `refund_orders`;
DROP TABLE IF EXISTS `pay_flows`;
ALTER TABLE `orders` DROP COLUMN `refund_time`;
