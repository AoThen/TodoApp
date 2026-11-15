package db

import (
	"database/sql"
	"fmt"
)

// TransactionFunc 是事务处理函数的类型定义
type TransactionFunc func(tx *sql.Tx) error

// WithTransaction 执行事务并自动处理提交/回滚
func WithTransaction(fn TransactionFunc) error {
	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			// 发生panic时回滚
			tx.Rollback()
			panic(p)
		} else if err != nil {
			// 发生错误时回滚
			tx.Rollback()
		} else {
			// 正常情况提交
			err = tx.Commit()
		}
	}()

	err = fn(tx)
	return err
}

// WithTransactionReturn 执行事务并返回结果，同时自动处理提交/回滚
func WithTransactionReturn[T any](fn func(tx *sql.Tx) (T, error)) (T, error) {
	var result T

	err := WithTransaction(func(tx *sql.Tx) error {
		res, err := fn(tx)
		if err != nil {
			return err
		}
		result = res
		return nil
	})

	return result, err
}
