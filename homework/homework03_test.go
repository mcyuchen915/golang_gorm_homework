package homework

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"gorm.io/gorm"
)

/*
题目1：模型定义
假设你要开发一个博客系统，有以下几个实体： User （用户）、 Post （文章）、 Comment （评论）。
要求 ：
使用Gorm定义 User 、 Post 和 Comment 模型，其中 User 与 Post 是一对多关系（一个用户可以发布多篇文章）， Post 与 Comment 也是一对多关系（一篇文章可以有多个评论）。
编写Go代码，使用Gorm创建这些模型对应的数据库表。
题目2：关联查询
基于上述博客系统的模型定义。
要求 ：
编写Go代码，使用Gorm查询某个用户发布的所有文章及其对应的评论信息。
编写Go代码，使用Gorm查询评论数量最多的文章信息。
题目3：钩子函数
继续使用博客系统的模型。
要求 ：
为 Post 模型添加一个钩子函数，在文章创建时自动更新用户的文章数量统计字段。
为 Comment 模型添加一个钩子函数，在评论删除时检查文章的评论数量，如果评论数量为 0，则更新文章的评论状态为 "无评论"。
*/

// 实体模型定义
// User 模型
type User struct {
	ID        uint      `gorm:"primaryKey"`                    // 主键，自增
	Email     string    `gorm:"size:128;uniqueIndex;not null"` // 邮箱，唯一索引，非空
	Name      string    `gorm:"size:64;not null"`              // 用户名，非空
	PostCount int64     `gorm:"default:0"`                     // 文章数量统计字段（用于钩子更新）
	Posts     []Post    `gorm:"foreignKey:UserID"`             // User 与 Post 一对多
	CreatedAt time.Time // 创建时间，GORM 自动管理
	UpdatedAt time.Time // 更新时间，GORM 自动管理
}

type auditField struct {
	DeletedBy string `gorm:"embedded"`
}
// Post 模型
type Post struct {
	ID            uint      		`gorm:"primaryKey"`
	Title         string    		`gorm:"size:128;not null"`
	Content       string    		`gorm:"type:text"`
	UserID        uint      		`gorm:"index;not null"`          	// 外键指向 User.ID
	CommentStatus string   			`gorm:"size:32;default:'无评论'"` 	// 评论状态字段
	Comments      []Comment 		`gorm:"foreignKey:PostID"`      	// Post 与 Comment 一对多
	Audit         auditField		`gorm:"embedded"`
	DeletedAt 	  gorm.DeletedAt    `gorm:"index"`
	CreatedAt	  time.Time 										// 创建时间，GORM 自动管理
	UpdatedAt 	  time.Time 										// 更新时间，GORM 自动管理
}

// Comment 模型
type Comment struct {
	ID      	uint   			`gorm:"primaryKey"`
	Content 	string 			`gorm:"type:text;not null"`
	PostID  	uint   			`gorm:"index;not null"` 					// 文章ID，普通索引，非空（外键关联 Post）
	UserID  	uint   			`gorm:"index;not null"` 					// 用户ID，普通索引，非空（外键关联 User）
	Audit       auditField		`gorm:"embedded"`
	DeletedAt 	gorm.DeletedAt  `gorm:"index"`
	CreatedAt 	time.Time 									// 创建时间，GORM 自动管理
	UpdatedAt 	time.Time 									// 更新时间，GORM 自动管理
}
// 自动迁移
func autoMigrate(t *testing.T,	tbname string) {
	db := NewTestDB(t, tbname)
	if err := db.AutoMigrate(&User{}, &Post{}, &Comment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	db.Exec("PRAGMA foreign_keys = OFF")
	db.Exec("DELETE FROM user")
	db.Exec("DELETE FROM post")
	db.Exec("DELETE FROM comment")
	db.Exec("PRAGMA foreign_keys = ON")
	t.Logf("migrate success")
}


// BeforeCreate Post 创建钩子：自动更新所属用户的文章数量统计字段
// 使用 gorm.Expr 原子自增，避免并发下"读-改-写"造成的计数丢失
func (p *Post) BeforeCreate(tx *gorm.DB) error {
	if tx == nil || tx.Error != nil {
		return tx.Error
	}
	return tx.Model(&User{}).
		Where("id = ?", p.UserID).
		UpdateColumn("post_count", gorm.Expr("post_count + 1")).
		Error
}

// AfterDelete Comment 删除钩子：检查文章剩余评论数量，为 0 时更新评论状态为"无评论"
// 通过数据库 COUNT 实时统计剩余评论，避免依赖被删除记录的内存状态
func (c *Comment) AfterDelete(tx *gorm.DB) error {
	if tx == nil || tx.Error != nil {
		return tx.Error
	}

	var remaining int64
	if err := tx.Model(&Comment{}).
		Where("post_id = ?", c.PostID).
		Count(&remaining).Error; err != nil {
		return err
	}

	status := "有评论"
	if remaining == 0 {
		status = "无评论"
	}

	return tx.Model(&Post{}).
		Where("id = ?", c.PostID).
		UpdateColumn("comment_status", status).
		Error
}

// TestAssociationQueries 题目2：关联查询
// 1. 查询某个用户发布的所有文章及其对应的评论信息（Preload 预加载，避免 N+1 查询）
// 2. 查询评论数量最多的文章信息（子查询统计评论数）
func TestAssociationQueries(t *testing.T) {
	tbname := "blog_queries.db"
	autoMigrate(t, tbname)
	db := NewTestDB(t, tbname)

	// 准备测试数据
	user := User{Email: "alice@example.com", Name: "alice"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	posts := []Post{
		{Title: "post1", Content: "content1", UserID: user.ID},
		{Title: "post2", Content: "content2", UserID: user.ID},
	}
	if err := db.Create(&posts).Error; err != nil {
		t.Fatalf("create posts: %v", err)
	}
	comments := []Comment{
		{Content: "c1", PostID: posts[0].ID, UserID: user.ID},
		{Content: "c2", PostID: posts[0].ID, UserID: user.ID},
	}
	if err := db.Create(&comments).Error; err != nil {
		t.Fatalf("create comments: %v", err)
	}

	// 1. 查询某个用户发布的所有文章及其对应的评论信息
	var userPosts []Post
	if err := db.Preload("Comments").
		Where("user_id = ?", user.ID).
		Find(&userPosts).Error; err != nil {
		t.Fatalf("query user posts: %v", err)
	}
	for _, p := range userPosts {
		t.Logf("post: %s, comments: %d", p.Title, len(p.Comments))
	}
	fmt.Println(userPosts)
	// 2. 查询评论数量最多的文章信息
	var topPost Post
	subQuery := db.Model(&Comment{}).
		Select("post_id").
		Group("post_id").
		Order("COUNT(*) DESC").
		Limit(1)
	if err := db.Where("id = (?)", subQuery).
		First(&topPost).Error; err != nil {
		t.Fatalf("query top post: %v", err)
	}
	t.Logf("top post: %s", topPost.Title)
	fmt.Println(topPost)
}

// 删除评论
func TestDeleteComment(t *testing.T) {
	tbname := "blog_delete.db"
	autoMigrate(t, tbname)
	db := NewTestDB(t, tbname)
	if err := db.AutoMigrate(&User{}, &Post{}, &Comment{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	//准备测试数据
	user := User{Email: "alice@example.com", Name: "alice"}	
	if err := db.Create(&user).Error; err != nil {	
		t.Fatalf("create user: %v", err)
	}
	posts := []Post{
		{Title: "post1", Content: "content1", UserID: user.ID},
		{Title: "post2", Content: "content2", UserID: user.ID},
	}
	if err := db.Create(&posts).Error; err != nil {
		t.Fatalf("create posts: %v", err)
	}
	comments := []Comment{
		{Content: "c1", PostID: posts[0].ID, UserID: user.ID},
		{Content: "c2", PostID: posts[0].ID, UserID: user.ID},
	}
	if err := db.Create(&comments).Error; err != nil {
		t.Fatalf("create comments: %v", err)
	}	
	// 删除评论
	// 删除前先更新DeletedBy字段
	if err := db.Model(&Comment{}).
		Where("id = ?", &comments[0].ID).
		Update("deleted_by", strconv.FormatUint(uint64(comments[0].UserID),	10)).Error; err != nil {
		t.Fatalf("update deleted_by: %v", err)
	}
	// 执行软删除，AfterDelete 钩子会被触发
	if err := db.Delete(&comments[0]).Error; err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	
	// 查询已删除的记录
	var check Comment
	if err := db.Unscoped().Where("id = ?", comments[0].ID).First(&check).Error; err != nil {
		t.Fatalf("query deleted comment: %v", err)
	}
	// 验证 deleted_at 字段已设置
	if check.DeletedAt.Time.IsZero() {
		t.Error("expected deleted_at to be set, but it's zero")
	}
	// 验证deleted_by字段被更新
	if check.Audit.DeletedBy != strconv.FormatUint(uint64(check.UserID), 10) {
		t.Fatalf("deleted_by not match")
	}
	// 硬删除
	if err := db.Unscoped().Delete(&check).Error; err != nil {
		t.Fatalf("delete comment: %v", err)
	}
	// 验证硬删除
	var count int64
	if err := db.Unscoped().Model(&Comment{}).Where("id = ?", check.ID).Count(&count).Error; err != nil {
		t.Fatalf("query comment count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 comment, but got %d", count)
	}
}
