# Mysql 性能解析分析器

自动分析多条 SQL 执行计划是否需要进行优化

# config
读取当前目录下的以下文件
input : 
1. 配置 `check.sql` 需要检查的 Select SQL
2. 配置 `mysql-connect.json` 的 Mysql 连接参数

output:
1. 输出 `mysql`

# Output 分析解析
```txt
执行的 SQL = Select * From go_users Where id = 1
----------------------------
执行计划：
Select ID: 1
查询耗时(ms) : 1.00
----------------------------
表扫描情况:
Table Name: go_users
访问类型 Access Type: const
扫描的行数 Rows Examined Per Scan: 1
生产的行数 Rows Produced Per Join: 1
过滤的百分比 Filtered: 100.00
----------------------------
Cost Info:
读取消耗ms Read Cost: 0.00
执行消耗 Eval Cost: 0.10
预消耗 Prefix Cost: 0.00
数据读取/join Data Read Per Join: 56
访问类型为 const，常量查找 by primary key/unique key. 针对性一条条找. 已经是最优访问方式，无需优化。
----------------------------
Used Columns:
id
created_at
updated_at
deleted_at
name
email


```
