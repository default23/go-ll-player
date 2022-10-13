package player

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/maxatome/go-testdeep/td"
)

func TestNewPlayer(t *testing.T) {
	t.Run("empty playlist", func(t *testing.T) {
		pl, _ := NewPlayer()

		td.CmpNil(t, pl.head, "head is empty")
		td.CmpNil(t, pl.tail, "tail is empty")
		td.CmpNil(t, pl.current, "no current song")
	})

	t.Run("predefined songs", func(t *testing.T) {
		sg, _ := NewSong("Сектор Газа - 30 лет", 30*time.Second)
		ap, _ := NewSong("Александр Пушной - Почему я идиот?", 1_1*time.Second)
		shuff, _ := NewSong("Михаил Шуфутинский - 3 сентября", 3*time.Second)

		pl, _ := NewPlayer(sg, ap, shuff)

		td.Cmp(t, *pl.head.song, sg, "сектор газа - первая песня")
		td.Cmp(t, *pl.tail.song, shuff, "шуфутинский - в конце")

		curr := pl.current
		td.Cmp(t, *curr.song, sg, "сектор газа - первый")
		td.CmpNil(t, curr.prev, "у первого элемента нет ссылки на пред элемент")

		curr = curr.next
		td.Cmp(t, *curr.song, ap, "пушной - второй")
		td.Cmp(t, *curr.prev.song, sg, "предыдущий(0) - сектор газа")

		curr = curr.next
		td.Cmp(t, *curr.song, shuff, "шуф - третий")
		td.Cmp(t, *curr.prev.song, ap, "предыдущий - пушной")
		td.CmpNil(t, curr.next, "3(последний) элемент не имеет ссылки на следующий")
	})
}

func TestPlayerImpl_AddSong(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		song, _ := NewSong("some song", time.Second)
		pl, _ := NewPlayer(song)

		anotherSong, _ := NewSong("another song", time.Second)
		_ = pl.AddSong(context.Background(), anotherSong)
		td.Cmp(t, *pl.tail.song, anotherSong, "новая песня должна быть добавлена в конец")
		td.Cmp(t, *pl.tail.prev.song, song, "предыдущая песня должна быть 'some song'")
		td.Cmp(t, *pl.head.song, song, "head должен указывать на первую песню")

		someSong, _ := NewSong("some another song", time.Second)
		_ = pl.AddSong(context.Background(), someSong)
		td.Cmp(t, *pl.tail.song, someSong, "новая песня должна быть добавлена в конец")
		td.Cmp(t, *pl.tail.prev.song, anotherSong, "предыдущая песня должна быть 'another song'")
		td.Cmp(t, *pl.head.song, song, "head должен указывать на первую песню")
	})

	t.Run("concurrent", func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Add(100_000)

		pl, _ := NewPlayer()
		for i := 0; i < 100_000; i++ {
			go func(el int) {
				s, _ := NewSong(fmt.Sprintf("%d", el), time.Second)
				_ = pl.AddSong(context.Background(), s)

				wg.Done()
			}(i)
		}
		wg.Wait()

		count := 1
		curr := pl.current
		for curr.next != nil {
			count++
			curr = curr.next
		}

		td.Cmp(t, count, 100_000)
	})
}

func TestPlaying(t *testing.T) {
	sg := Song{Name: "Сектор Газа - 30 лет", Duration: 30 * time.Second}
	ap := Song{Name: "Александр Пушной - Почему я идиот?", Duration: 30 * time.Second}
	shuff := Song{Name: "Михаил Шуфутинский - 3 сентября", Duration: 30 * time.Second}

	pl, _ := NewPlayer(sg, ap, shuff)

	t.Run("should stop player", func(t *testing.T) {
		for i := 0; i < 100; i++ {
			pl.current = pl.head
			ctx, cancel := context.WithCancel(context.Background())

			_ = pl.Play(ctx)

			time.Sleep(10 * time.Millisecond)
			cancel()

			time.Sleep(time.Millisecond)
			td.Cmp(t, pl.playedTime, time.Duration(0), "playedTime должен быть 0")
			td.CmpFalse(t, pl.isPlaying, "воспроизведение остановлено")
		}
	})

	t.Run("should pause player", func(t *testing.T) {
		ctx := context.Background()
		pl.current = pl.head

		_ = pl.Play(ctx)

		time.Sleep(100 * time.Millisecond)
		_ = pl.Pause(ctx)

		time.Sleep(time.Millisecond)
		td.CmpGte(t, pl.playedTime.Milliseconds(), (100 * time.Millisecond).Milliseconds(), "playedTime должен быть больше 100мс")
		td.CmpLte(t, pl.playedTime.Milliseconds(), (102 * time.Millisecond).Milliseconds(), "playedTime должен быть меньше 101мс")
		td.CmpFalse(t, pl.isPlaying, "воспроизведение остановлено")

		_ = pl.Play(ctx)
		td.CmpTrue(t, pl.isPlaying, "воспроизведение продолжено")

		time.Sleep(100 * time.Millisecond)
		_ = pl.Pause(ctx)

		time.Sleep(time.Millisecond)
		td.CmpGte(t, pl.playedTime.Milliseconds(), (200 * time.Millisecond).Milliseconds(), "playedTime должен быть больше 200мс")
		td.CmpLte(t, pl.playedTime.Milliseconds(), (202 * time.Millisecond).Milliseconds(), "playedTime должен быть меньше 202мс")
		td.CmpFalse(t, pl.isPlaying, "воспроизведение остановлено")
	})

	t.Run("SHOULD NOT play on empty playlist", func(t *testing.T) {
		emptyPl, _ := NewPlayer()
		_ = emptyPl.Play(context.Background())

		td.CmpFalse(t, emptyPl.isPlaying, "не должен начать воспроизведение")
	})

	t.Run("should play next song on duration is over", func(t *testing.T) {
		sg := Song{Name: "Сектор Газа - 30 лет", Duration: 100 * time.Millisecond}
		ap := Song{Name: "Александр Пушной - Почему я идиот?", Duration: 30 * time.Second}

		ctx := context.Background()
		nextPl, _ := NewPlayer(sg, ap)
		_ = nextPl.Play(ctx)
		time.Sleep(150 * time.Millisecond)
		_ = nextPl.Pause(ctx)

		time.Sleep(time.Millisecond)
		td.CmpFalse(t, nextPl.isPlaying)
		td.Cmp(t, nextPl.current.song, &ap, "текущая песня должна быть 'Александр Пушной - Почему я идиот?'")
	})

	t.Run("should stop player and move cursor at first song if the last song is over", func(t *testing.T) {
		sg := Song{Name: "Сектор Газа - 30 лет", Duration: 100 * time.Millisecond}
		ap := Song{Name: "Александр Пушной - Почему я идиот?", Duration: 100 * time.Millisecond}

		ctx := context.Background()
		nextPl, _ := NewPlayer(sg, ap)

		nextPl.current = nextPl.tail
		_ = nextPl.Play(ctx)
		time.Sleep(150 * time.Millisecond)

		td.CmpFalse(t, nextPl.isPlaying)
		td.Cmp(t, nextPl.current.song, &sg, "текущая песня должна быть 'Сектор Газа - 30 лет'")
	})
}
