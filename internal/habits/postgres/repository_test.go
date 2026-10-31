package postgres

import (
	"context"
	"errors"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"plans-tracker-backend/internal/habits"
)

func TestListFiltersUserAndLoadsHabit(t *testing.T) {
	pool, ctx := openHabitsTestPool(t)

	var userID, otherUserID string
	if err := pool.QueryRow(ctx, "SELECT gen_random_uuid()::text, gen_random_uuid()::text").Scan(&userID, &otherUserID); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		for _, id := range []string{userID, otherUserID} {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM habits WHERE user_id = $1", id); err != nil {
				t.Errorf("clean habits: %v", err)
			}
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM habit_categories WHERE user_id = $1", id); err != nil {
				t.Errorf("clean categories: %v", err)
			}
		}
	}()

	var categoryID, oldID, newID string
	if err := pool.QueryRow(ctx,
		"INSERT INTO habit_categories (user_id, label) VALUES ($1, 'Health') RETURNING id", userID,
	).Scan(&categoryID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO habits (user_id, title, category_id, repeat_rule, created_at)
		VALUES ($1, 'Older', $2, '{"type":"weekly","daysOfWeek":[1,3]}', '2026-10-01T12:00:00Z')
		RETURNING id
	`, userID, categoryID).Scan(&oldID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		INSERT INTO habits (user_id, title, repeat_rule, start_minute, duration_minutes, created_at)
		VALUES ($1, 'Newer', '{"type":"daily"}', 600, 30, '2026-10-02T12:00:00Z')
		RETURNING id
	`, userID).Scan(&newID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO habits (user_id, title, repeat_rule)
		VALUES ($1, 'Other user', '{"type":"daily"}')
	`, otherUserID); err != nil {
		t.Fatal(err)
	}

	listed, err := NewRepository(pool).List(ctx, userID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 2 || listed[0].Habit.ID != newID || listed[1].Habit.ID != oldID {
		t.Fatalf("unexpected habit order or user filtering: %#v", listed)
	}
	if listed[0].Habit.StartMinute == nil || *listed[0].Habit.StartMinute != 600 || listed[0].Habit.DurationMinutes != 30 || listed[0].Habit.RepeatRule.Type != "daily" {
		t.Fatalf("new habit fields: %#v", listed[0])
	}
	if listed[1].Habit.CategoryID == nil || *listed[1].Habit.CategoryID != categoryID || len(listed[1].Habit.RepeatRule.DaysOfWeek) != 2 {
		t.Fatalf("old habit fields: %#v", listed[1])
	}

	withCategory, err := NewRepository(pool).ByID(ctx, userID, oldID)
	if err != nil || withCategory.Category == nil || withCategory.Category.ID != categoryID || withCategory.Category.Color != nil || withCategory.Category.SortOrder != 0 {
		t.Fatalf("habit details with category: %#v, %v", withCategory, err)
	}
	withoutCategory, err := NewRepository(pool).ByID(ctx, userID, newID)
	if err != nil || withoutCategory.Category != nil {
		t.Fatalf("habit details without category: %#v, %v", withoutCategory, err)
	}
	if _, err := NewRepository(pool).ByID(ctx, otherUserID, oldID); !errors.Is(err, habits.ErrNotFound) {
		t.Fatalf("another user's habit: %v", err)
	}
}

func TestHabitCategoryAndCompletionLifecycle(t *testing.T) {
	pool, ctx := openHabitsTestPool(t)
	var userID, otherUserID string
	if err := pool.QueryRow(ctx, "SELECT gen_random_uuid()::text, gen_random_uuid()::text").Scan(&userID, &otherUserID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
		defer stop()
		for _, id := range []string{userID, otherUserID} {
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM habits WHERE user_id = $1", id); err != nil {
				t.Errorf("clean habits: %v", err)
			}
			if _, err := pool.Exec(cleanupCtx, "DELETE FROM habit_categories WHERE user_id = $1", id); err != nil {
				t.Errorf("clean categories: %v", err)
			}
		}
	})
	repo := NewRepository(pool)
	category, err := repo.CreateCategory(ctx, userID, habits.CategoryInput{Label: "Health"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.CreateCategory(ctx, userID, habits.CategoryInput{Label: "Study"})
	if err != nil || second.SortOrder != category.SortOrder+1 {
		t.Fatalf("category order: %#v, %v", second, err)
	}
	categoryList, err := repo.CategoryList(ctx, userID)
	if err != nil || len(categoryList) != 2 {
		t.Fatalf("category list: %#v, %v", categoryList, err)
	}
	category, err = repo.PatchCategory(ctx, userID, category.ID, habits.CategoryPatch{Label: habits.Field[string]{Set: true, Value: "Wellness"}})
	if err != nil || category.Label != "Wellness" {
		t.Fatalf("category patch: %#v, %v", category, err)
	}
	if _, err := repo.CategoryByID(ctx, otherUserID, category.ID); !errors.Is(err, habits.ErrNotFound) {
		t.Fatalf("foreign category: %v", err)
	}

	created, err := repo.Create(ctx, userID, habits.HabitInput{
		Title: "Walk", CategoryID: &category.ID, RepeatRule: habits.RepeatRule{Type: habits.RepeatDaily}, DurationMinutes: 60,
	})
	if err != nil || created.Category == nil || created.Category.Label != "Wellness" {
		t.Fatalf("create: %#v, %v", created, err)
	}
	if _, err := repo.Create(ctx, otherUserID, habits.HabitInput{
		Title: "Cross user", CategoryID: &category.ID, RepeatRule: habits.RepeatRule{Type: habits.RepeatDaily}, DurationMinutes: 60,
	}); !errors.Is(err, habits.ErrInvalidReference) {
		t.Fatalf("cross-user category accepted: %v", err)
	}
	if _, err := repo.ByID(ctx, otherUserID, created.Habit.ID); !errors.Is(err, habits.ErrNotFound) {
		t.Fatalf("foreign habit: %v", err)
	}

	date := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	marked, err := repo.SetCompletion(ctx, userID, created.Habit.ID, date, true)
	if err != nil || marked == nil {
		t.Fatalf("mark: %#v, %v", marked, err)
	}
	markedAgain, err := repo.SetCompletion(ctx, userID, created.Habit.ID, date, true)
	if err != nil || markedAgain == nil || markedAgain.ID != marked.ID {
		t.Fatalf("repeat mark: %#v, %v", markedAgain, err)
	}
	if _, err := repo.SetCompletion(ctx, otherUserID, created.Habit.ID, date, true); !errors.Is(err, habits.ErrNotFound) {
		t.Fatalf("foreign completion: %v", err)
	}
	listed, err := repo.CompletionList(ctx, userID, habits.CompletionFilter{StartDate: &date, EndDate: &date})
	if err != nil || len(listed) != 1 || listed[0].HabitID != created.Habit.ID {
		t.Fatalf("completion list: %#v, %v", listed, err)
	}
	cleared, err := repo.SetCompletion(ctx, userID, created.Habit.ID, date, false)
	if err != nil || cleared != nil {
		t.Fatalf("clear: %#v, %v", cleared, err)
	}
	listed, err = repo.CompletionList(ctx, userID, habits.CompletionFilter{})
	if err != nil || len(listed) != 0 {
		t.Fatalf("completions after clear: %#v, %v", listed, err)
	}

	if err := repo.DeleteCategory(ctx, userID, category.ID); err != nil {
		t.Fatal(err)
	}
	uncategorized, err := repo.ByID(ctx, userID, created.Habit.ID)
	if err != nil || uncategorized.Habit.CategoryID != nil || uncategorized.Category != nil {
		t.Fatalf("category deletion: %#v, %v", uncategorized, err)
	}
	updated, err := repo.Patch(ctx, userID, created.Habit.ID, habits.HabitPatch{Title: habits.Field[string]{Set: true, Value: "Evening walk"}})
	if err != nil || updated.Habit.Title != "Evening walk" {
		t.Fatalf("habit patch: %#v, %v", updated, err)
	}
	if err := repo.Delete(ctx, userID, created.Habit.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ByID(ctx, userID, created.Habit.ID); !errors.Is(err, habits.ErrNotFound) {
		t.Fatalf("deleted habit: %v", err)
	}
}

func openHabitsTestPool(t *testing.T) (*pgxpool.Pool, context.Context) {
	t.Helper()
	databaseURL := os.Getenv("HABITS_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set HABITS_TEST_DATABASE_URL to run the PostgreSQL integration test")
	}
	parsed, err := url.Parse(databaseURL)
	if err != nil || parsed.Path != "/habits_test" {
		t.Fatal("HABITS_TEST_DATABASE_URL must point to habits_test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	return pool, ctx
}
