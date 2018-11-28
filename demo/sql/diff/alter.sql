DROP TABLE IF EXISTS  `tx_parcel`;
CREATE TABLE `tx_parcel` (
  `id` bigint(20) NOT NULL COMMENT 'ID',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `modify_time` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0' COMMENT '逻辑删除:true=1,false=0',
  `logno` bigint(19) DEFAULT NULL COMMENT '日志编号',
  `user_id` bigint(20) DEFAULT NULL COMMENT '用户ID',
  `sender_id` bigint(20) DEFAULT NULL COMMENT '发件人ID',
  `warehouse` int(11) DEFAULT NULL COMMENT '数据所在仓库 WareHouse',
  `recver_id` bigint(20) DEFAULT NULL COMMENT '收件人ID',
  `track_num` varchar(50) NOT NULL COMMENT '运单号',
  `weight_pkg` decimal(10,2) DEFAULT NULL COMMENT '商家包裹总重量（a+b+m）',
  `weight_dim` decimal(10,2) DEFAULT NULL COMMENT '商家体积重',
  `store_time` datetime DEFAULT NULL COMMENT '最新入库时间',
  `shelf_time` datetime DEFAULT NULL COMMENT '最新上架时间',
  `leave_time` datetime DEFAULT NULL COMMENT '最新出库时间',
  `track_time` datetime DEFAULT NULL COMMENT '首个国内物流时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8 ROW_FORMAT=COMPACT COMMENT='包裹';

ALTER TABLE `tx_parcel`
  ADD INDEX `ix_user_id` (`sender_id` ASC),
  ADD INDEX `ix_sender_id` (`sender_id` ASC),
  ADD INDEX `ix_recver_idx` (`recver_id`),
  ADD UNIQUE `uq_trknum` (`track_num` ASC);

DROP TRIGGER IF EXISTS `tx_parcel$log$bu`;
DELIMITER $$
CREATE TRIGGER `tx_parcel$log$bu` BEFORE UPDATE ON `tx_parcel`
FOR EACH ROW BEGIN
  insert into `tx_parcel$log` select *, null, 3, now() from `tx_parcel` where id= OLD.id;
END $$
DELIMITER ;

DROP TRIGGER IF EXISTS `tx_parcel$log$bd`;
DROP TABLE IF EXISTS  `tx_track`;
