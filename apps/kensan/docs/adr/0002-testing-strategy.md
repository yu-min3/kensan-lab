# ADR-0002: テスト戦略 - MockRepositoryによるUnit Testのみ採用

**ステータス**: 承認済み
**日付**: 2026-01-12
**決定者**: Yu

---

## コンテキスト

マイクロサービスのテスト戦略として、以下の選択肢がある:

1. **Unit Test (MockRepository)**: Service層のビジネスロジックをテスト。Repositoryはモック化。
2. **Integration Test (実DB)**: Repository層のSQL実装をテスト。Docker PostgreSQLを使用。
3. **E2E Test**: 全層を結合してテスト。

一般的なベストプラクティスでは、テストピラミッドに従い全層をテストすることが推奨される。

```
      /\        E2E (少数)
     /  \
    /────\      Integration (中程度)
   /      \
  /────────\    Unit (多数)
```

## 決定

**Unit Test（MockRepositoryによるService層テスト）のみを実装する。**

Repository層のIntegration Testは実装しない。

## 理由

### 1. 開発体制

- 個人開発プロジェクトであり、開発速度を優先する
- テストの保守コストを最小限に抑える

### 2. 費用対効果

- Service層のUnit Testで、ビジネスロジックの大部分をカバーできる
- Repository層のSQLは比較的シンプルなCRUD操作が中心
- Integration Testの構築・保守コストに対して、得られるバグ検出効果が限定的

### 3. 現実的なリスク評価

- SQLの不具合は開発中のマニュアルテストで発見可能
- スキーマ変更時は目視でRepository実装を確認できる規模

## 採用するテスト構成

```
services/<name>/internal/
├── service/
│   └── service_test.go      # ✅ 実装済み (MockRepository)
└── repository/
    └── repository_test.go   # ❌ 実装しない
```

### テスト実装パターン

```go
// MockRepository で Service をテスト
func TestService_Create_Success(t *testing.T) {
    mockRepo := new(MockRepository)
    svc := New(mockRepo)

    mockRepo.On("Create", ctx, mock.AnythingOfType("*Entry")).Return(nil)

    result, err := svc.Create(ctx, userID, input)

    assert.NoError(t, err)
    mockRepo.AssertExpectations(t)
}

// バリデーションロジックのテスト（DBアクセスなし）
func TestService_Create_TitleRequired(t *testing.T) {
    mockRepo := new(MockRepository)
    svc := New(mockRepo)

    _, err := svc.Create(ctx, userID, &CreateInput{Title: ""})

    assert.ErrorIs(t, err, ErrTitleRequired)
    mockRepo.AssertNotCalled(t, "Create")
}
```

## 結果

### メリット

- テスト実行が高速（DBアクセスなし）
- CI/CDパイプラインがシンプル
- テストの保守コストが低い
- 並列実行が容易

### デメリット

- SQLの不具合を自動テストで検出できない
- DBスキーマとRepository実装の整合性は手動確認が必要
- 複雑なクエリ（JOIN、集計）のテストがない

### 将来の検討事項

以下の状況が発生した場合、Integration Testの追加を再検討する:

1. **チーム開発への移行**: 複数人での開発になり、SQL変更の影響範囲把握が困難になる場合
2. **複雑なクエリの増加**: 集計、検索など、SQLの正確性が重要なクエリが増えた場合
3. **本番障害**: SQLの不具合に起因する障害が発生した場合

## 参考

- [TestDouble - Martin Fowler](https://martinfowler.com/bliki/TestDouble.html)
- [Test Pyramid](https://martinfowler.com/articles/practical-test-pyramid.html)
- [testify/mock](https://github.com/stretchr/testify#mock-package)
