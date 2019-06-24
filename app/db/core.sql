/*
Navicat MySQL Data Transfer

Source Server         : localhost
Source Server Version : 80015
Source Host           : localhost:3306
Source Database       : intranet_striker

Target Server Type    : MYSQL
Target Server Version : 80015
File Encoding         : 65001

Date: 2019-03-10 21:01:20
*/

SET FOREIGN_KEY_CHECKS=0;

-- ----------------------------
-- Table structure for beacon
-- ----------------------------
DROP TABLE IF EXISTS `beacon`;
CREATE TABLE `beacon` (
  `guid` tinyblob NOT NULL,
  `type` tinyint(3) unsigned NOT NULL DEFAULT '0',
  `network` tinyblob NOT NULL,
  `address` tinyblob NOT NULL,
  `add_time` bigint(20) NOT NULL DEFAULT '0',
  PRIMARY KEY (`guid`(40))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ----------------------------
-- Table structure for beacon_key
-- ----------------------------
DROP TABLE IF EXISTS `beacon_key`;
CREATE TABLE `beacon_key` (
  `guid` tinyblob NOT NULL,
  `aes_key` tinyblob NOT NULL,
  `aes_iv` tinyblob NOT NULL,
  `ecdsa_publickey` blob NOT NULL,
  PRIMARY KEY (`guid`(40))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ----------------------------
-- Table structure for beacon_syncer
-- ----------------------------
DROP TABLE IF EXISTS `beacon_syncer`;
CREATE TABLE `beacon_syncer` (
  `guid` tinyblob NOT NULL,
  `controller_send` bigint(20) unsigned NOT NULL,
  `beacon_receive` bigint(20) unsigned NOT NULL,
  `beacon_send` bigint(20) unsigned NOT NULL,
  `controller_receive` bigint(20) unsigned NOT NULL,
  `update_time` bigint(20) NOT NULL,
  PRIMARY KEY (`guid`(40))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ----------------------------
-- Table structure for bootstrap
-- ----------------------------
DROP TABLE IF EXISTS `bootstrap`;
CREATE TABLE `bootstrap` (
  `tag` tinyblob NOT NULL,
  `mode` tinyint(3) unsigned NOT NULL,
  `target` blob NOT NULL,
  `config` blob NOT NULL,
  `interval` bigint(20) NOT NULL,
  `enable` tinyint(1) NOT NULL,
  `update_time` bigint(20) NOT NULL,
  `add_time` bigint(20) NOT NULL,
  PRIMARY KEY (`tag`(255))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ----------------------------
-- Table structure for dns_client
-- ----------------------------
DROP TABLE IF EXISTS `dns_client`;
CREATE TABLE `dns_client` (
  `tag` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL,
  `protocol` tinyint(3) unsigned NOT NULL,
  `config` varchar(1024) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL,
  PRIMARY KEY (`tag`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ----------------------------
-- Table structure for node
-- ----------------------------
DROP TABLE IF EXISTS `node`;
CREATE TABLE `node` (
  `guid` tinyblob NOT NULL,
  `type` tinyint(3) unsigned NOT NULL,
  `network` tinyblob NOT NULL,
  `address` tinyblob NOT NULL,
  `is_bootstrap` tinyint(1) NOT NULL,
  `add_time` bigint(20) NOT NULL,
  PRIMARY KEY (`guid`(40))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ----------------------------
-- Table structure for node_key
-- ----------------------------
DROP TABLE IF EXISTS `node_key`;
CREATE TABLE `node_key` (
  `guid` tinyblob NOT NULL,
  `aes_key` tinyblob NOT NULL,
  `aes_iv` tinyblob NOT NULL,
  `ecdsa_publickey` blob NOT NULL,
  PRIMARY KEY (`guid`(40))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ----------------------------
-- Table structure for node_syncer
-- ----------------------------
DROP TABLE IF EXISTS `node_syncer`;
CREATE TABLE `node_syncer` (
  `guid` tinyblob NOT NULL,
  `controller_send` bigint(20) unsigned NOT NULL,
  `node_receive` bigint(20) unsigned NOT NULL,
  `node_send` bigint(20) unsigned NOT NULL,
  `controller_receive` bigint(20) unsigned NOT NULL,
  `update_time` bigint(20) NOT NULL,
  PRIMARY KEY (`guid`(40))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

-- ----------------------------
-- Table structure for ntp_client
-- ----------------------------
DROP TABLE IF EXISTS `ntp_client`;
CREATE TABLE `ntp_client` (
  `tag` varchar(32) CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci NOT NULL,
  `config` blob NOT NULL,
  PRIMARY KEY (`tag`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;
