package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

import (
	_ "github.com/go-sql-driver/mysql"
)

// 分析输出结果 file
const outputAnalysisFileName = "mysql-analysis-output.txt"

// 需要检查的 SQL 文件. 当前目录下
const checkSqlFileName = "check.sql"

type CostInfo struct {
	QueryCost       string `json:"query_cost"`
	ReadCost        string `json:"read_cost"`
	EvalCost        string `json:"eval_cost"`
	PrefixCost      string `json:"prefix_cost"`
	DataReadPerJoin string `json:"data_read_per_join"`
}

type TableInfo struct {
	TableName           string   `json:"table_name"`
	AccessType          string   `json:"access_type"`
	RowsExaminedPerScan int64    `json:"rows_examined_per_scan"`
	RowsProducedPerJoin int64    `json:"rows_produced_per_join"`
	Filtered            string   `json:"filtered"`
	CostInfo            CostInfo `json:"cost_info"`
	UsedColumns         []string `json:"used_columns"`
}

type QueryBlock struct {
	SelectID int64     `json:"select_id"`
	CostInfo CostInfo  `json:"cost_info"`
	Table    TableInfo `json:"table"`
}

type ExplainResult struct {
	QueryBlock QueryBlock `json:"query_block"`
}

func parseExplainJSON(explainJSON string) (*ExplainResult, error) {
	var explainResult ExplainResult
	err := json.Unmarshal([]byte(explainJSON), &explainResult)
	if err != nil {
		return nil, err
	}
	return &explainResult, nil
}

func readMysqlConfigMap(filename string) (map[string]string, error) {
	var config map[string]string

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

func splitSQLStatements(sql string) []string {
	statements := make([]string, 0)
	scanner := bufio.NewScanner(strings.NewReader(sql))

	// 设置 Scanner 的分隔函数，以分号 `;` 作为分隔符
	splitFunc := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		for i := 0; i < len(data); i++ {
			if data[i] == ';' {
				return i + 1, data[:i], nil
			}
		}
		if atEOF && len(data) > 0 {
			return len(data), data, nil
		}
		return 0, nil, nil
	}
	scanner.Split(splitFunc)

	for scanner.Scan() {
		statement := strings.TrimSpace(scanner.Text())
		if statement != "" {
			statements = append(statements, statement)
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}

	return statements
}

func main() {
	// MySQL数据库连接信息
	dsnTemplate := NewKvTemplate("${username}:${password}@tcp(${host}:${port})/${database}")
	// 替换占位符的值
	configMap, err2 := readMysqlConfigMap("mysql-connect.json")
	if err2 != nil {
		log.Fatal("读取 mysql-connect.json 数据错误!")
		return
	}
	dsn := dsnTemplate.Render(configMap)

	// 建立数据库连接
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 读取当前目录下的check.sql文件

	sqlBytes, err := ioutil.ReadFile(checkSqlFileName)
	if err != nil {
		log.Fatal("无法读取check.sql文件：", err)
	}

	// 将SQL文件内容转换为字符串
	sqlQuery := string(sqlBytes)

	sqlArray := splitSQLStatements(sqlQuery)

	// 打开文件用于写入结果
	file, err := os.Create(outputAnalysisFileName)
	if err != nil {
		log.Fatal("无法创建结果文件：", err)
	}
	defer file.Close()

	// 将标准输出重定向到文件
	log.SetOutput(file)

	for _, selectSql := range sqlArray {

		// 使用EXPLAIN FORMAT=json获取执行计划
		querySql := fmt.Sprintf("EXPLAIN FORMAT=json %s", selectSql)
		rows, err := db.Query(querySql)
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()

		var explainJSON string
		for rows.Next() {
			if err := rows.Scan(&explainJSON); err != nil {
				log.Fatal(err)
			}
		}

		fmt.Fprintf(file, "执行的 SQL = %s \n", selectSql)

		// 解析执行计划
		explainResult, err := parseExplainJSON(explainJSON)
		if err != nil {
			log.Fatal(err)
		}

		// 宏观信息
		fmt.Fprintf(file, "---------------------------- \n")
		fmt.Fprintf(file, "执行计划：\n")
		fmt.Fprintf(file, "Select ID: %d\n", explainResult.QueryBlock.SelectID)
		fmt.Fprintf(file, "查询耗时(ms) : %s\n", explainResult.QueryBlock.CostInfo.QueryCost)

		// 物理计划执行情况
		fmt.Fprintf(file, "---------------------------- \n")
		fmt.Fprintf(file, "表扫描情况: \n")
		fmt.Fprintf(file, "Table Name: %s\n", explainResult.QueryBlock.Table.TableName)
		fmt.Fprintf(file, "访问类型 Access Type: %s\n", explainResult.QueryBlock.Table.AccessType)
		fmt.Fprintf(file, "扫描的行数 Rows Examined Per Scan: %d\n", explainResult.QueryBlock.Table.RowsExaminedPerScan)
		fmt.Fprintf(file, "生产的行数 Rows Produced Per Join: %d\n", explainResult.QueryBlock.Table.RowsProducedPerJoin)
		fmt.Fprintf(file, "过滤的百分比 Filtered: %s\n", explainResult.QueryBlock.Table.Filtered)

		// 成本消耗
		fmt.Fprintf(file, "---------------------------- \n")
		fmt.Fprintf(file, "Cost Info: \n")
		fmt.Fprintf(file, "读取消耗ms Read Cost: %s\n", explainResult.QueryBlock.Table.CostInfo.ReadCost)
		fmt.Fprintf(file, "执行消耗 Eval Cost: %s\n", explainResult.QueryBlock.Table.CostInfo.EvalCost)
		fmt.Fprintf(file, "预消耗 Prefix Cost: %s\n", explainResult.QueryBlock.Table.CostInfo.PrefixCost)
		fmt.Fprintf(
			file,
			"数据读取/join Data Read Per Join: %s\n",
			explainResult.QueryBlock.Table.CostInfo.DataReadPerJoin,
		)

		// 解释需要优化的点
		switch explainResult.QueryBlock.Table.AccessType {
		case "ALL":
			fmt.Fprintf(file, "访问类型为 ALL，全表扫描. 如果非必须, 需要索引。")
		case "index":
			fmt.Fprintf(file, "访问类型为 index，索引扫描. 可以再优化")
		case "range":
			fmt.Fprintf(file, "访问类型为 range，范围扫描. 可以考虑添加更适合的索引以提升性能。")
		case "ref":
			fmt.Fprintf(
				file,
				"访问类型为 ref，唯一索引查找. 使用非唯一性索引或唯一性索引查找匹配的数据行. 基本不需要优化.",
			)
		case "const":
			fmt.Fprintf(
				file,
				"访问类型为 const，常量查找 by primary key/unique key. 针对性一条条找. 已经是最优访问方式，无需优化。",
			)
		case "unique_subquery":
			fmt.Fprintf(
				file,
				"访问类型为 unique_subquery，唯一子查询. 在子查询中使用了唯一索引来查找匹配的数据行. 建议 join 优化掉子查询",
			)
		case "index_subquery":
			fmt.Fprintf(
				file,
				"访问类型为 index_subquery，在子查询中使用了非唯一性索引来查找匹配的数据行. 建议 join 优化",
			)

		default:
			fmt.Fprintf(file, "未知的访问类型，可以进一步分析执行计划并优化查询。")
		}

		if explainResult.QueryBlock.Table.RowsExaminedPerScan > 1000 {
			fmt.Fprintf(file, "扫描的行数较多，可能需要优化查询或添加索引。")
		}

		fmt.Fprintf(file, "\n----------------------------")
		fmt.Fprintf(file, "\nUsed Columns:")
		for _, col := range explainResult.QueryBlock.Table.UsedColumns {
			fmt.Fprintf(file, "\n - %s", col)
		}

		fmt.Fprintf(file, "\n\n ")
		fmt.Fprintf(file, "\n============================\n ")
		fmt.Fprintf(file, "\n\n ")
	}

}
