DROP TABLE IF EXISTS  `sys_schema_version`;
CREATE TABLE `sys_schema_version` (
  `version` BIGINT NOT NULL COMMENT '版本号',
  `created` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

REPLACE INTO sys_schema_version (version, created) VALUES( 2018111101, NOW());

-- -------------------------------------------
SELECT max(version) FROM sys_schema_version;

DROP TABLE IF EXISTS  `sys_hot_separation`;
CREATE TABLE `sys_hot_separation` (
  `table_name` VARCHAR(100) NOT NULL COMMENT '表名',
  `checked_id` BIGINT(20) NOT NULL COMMENT '检查过的最大ID',
  `checked_tm` DATETIME NOT NULL COMMENT '上次检查的时间',
  PRIMARY KEY (`table_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

REPLACE INTO sys_schema_version (version, created) VALUES( 2018111102, NOW());

-- -------------------------------------------
SELECT max(version) FROM sys_schema_version;

DROP TABLE IF EXISTS  `tx_parcel`;
CREATE TABLE `tx_parcel` (
  `id` bigint(20) NOT NULL COMMENT 'ID',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `modify_time` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0' COMMENT '逻辑删除:true=1,false=0',
  `logno` bigint(20) DEFAULT NULL COMMENT '日志编号',
  `user_id` bigint(20) DEFAULT NULL COMMENT '用户ID',
  `warehouse` int(11) DEFAULT NULL COMMENT '数据所在仓库 WareHouse',
  `sender_id` bigint(20) DEFAULT NULL COMMENT '发件人ID',
  `recver_id` bigint(20) DEFAULT NULL COMMENT '收件人ID',
  `track_num` varchar(50) NOT NULL COMMENT '运单号',
  `weight_pkg` decimal(10,2) DEFAULT NULL COMMENT '商家包裹总重量（a+b+m）',
  `weight_dim` decimal(10,2) DEFAULT NULL COMMENT '商家体积重',
  `input_time` datetime DEFAULT NULL COMMENT '录单时间',
  `store_time` datetime DEFAULT NULL COMMENT '最新入库时间',
  `shelf_time` datetime DEFAULT NULL COMMENT '最新上架时间',
  `leave_time` datetime DEFAULT NULL COMMENT '最新出库时间',
  `track_time` datetime DEFAULT NULL COMMENT '首个国内物流时间',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=COMPACT COMMENT='包裹';

DROP TABLE IF EXISTS  `tx_track`;
CREATE TABLE `tx_track` (
  `id` bigint(20) NOT NULL COMMENT 'ID',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `modify_time` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0' COMMENT '逻辑删除:true=1,false=0',
  `logno` bigint(20) DEFAULT NULL COMMENT '日志编号',
  `user_id` bigint(20) DEFAULT NULL COMMENT '用户ID',
  `parcel_id` bigint(20) NOT NULL COMMENT '包裹ID',
  `company` int(11) NOT NULL COMMENT '物流公司',
  `track_num` varchar(50) NOT NULL COMMENT '跟踪单号',
  `events` text NOT NULL COMMENT '事件json:[{date:xx,info:xx},]',
  `status` int(11) NOT NULL COMMENT '物流状态',
  `dest_city` varchar(50) DEFAULT NULL COMMENT '包裹目的地城市',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=COMPACT COMMENT='物流跟踪';

DROP TABLE IF EXISTS  `tx_parcel_event`;
CREATE TABLE `tx_parcel_event` (
  `id` bigint(20) NOT NULL COMMENT 'ID',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `modify_time` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0' COMMENT '逻辑删除:true=1,false=0',
  `logno` bigint(20) DEFAULT NULL COMMENT '日志编号',
  `user_id` bigint(20) DEFAULT NULL COMMENT '用户ID',
  `parcel_id` bigint(20) NOT NULL COMMENT '包裹ID',
  `type` int(11) NOT NULL COMMENT '事件类型：取件|入库|出库',
  `source` varchar(100) DEFAULT NULL COMMENT '发生地',
  `operator_id` bigint(20) DEFAULT NULL COMMENT '操作员ID',
  `is_closed` tinyint(1) DEFAULT NULL COMMENT '是否关闭:true=1,false=0',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=COMPACT COMMENT='包裹事件';

DROP TABLE IF EXISTS  `tx_receiver`;
CREATE TABLE `tx_receiver` (
  `id` bigint(20) NOT NULL COMMENT 'ID',
  `create_time` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `modify_time` datetime DEFAULT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `is_deleted` tinyint(1) NOT NULL DEFAULT '0' COMMENT '逻辑删除:true=1,false=0',
  `logno` bigint(20) DEFAULT NULL COMMENT '日志编号',
  `user_id` bigint(20) DEFAULT NULL COMMENT '用户ID',
  `name` varchar(20) NOT NULL COMMENT '姓名',
  `phone` varchar(60) NOT NULL COMMENT '电话',
  `postcode` varchar(20) NOT NULL COMMENT '邮编',
  `country` int(11) NOT NULL COMMENT '国家(代码)',
  `province` char(3) NOT NULL COMMENT '州/省/直辖市(简写)',
  `city` varchar(20) NOT NULL COMMENT '城市',
  `district` varchar(45) DEFAULT NULL COMMENT '县区',
  `address1` varchar(100) NOT NULL COMMENT '区/路/街',
  `address2` varchar(100) DEFAULT NULL COMMENT '楼/室',
  `hash` varchar(40) NOT NULL COMMENT '姓名，电话，省市区地址1，2的hash',
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 ROW_FORMAT=COMPACT COMMENT='收件人';

REPLACE INTO sys_schema_version (version, created) VALUES( 2018111103, NOW());
