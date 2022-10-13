package player

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

/*
Разработать музыкальный плеер
Должен содержать playlist для воспроизведения

Обладает следующими возможностями:
- Play - начинает воспроизведение
- Pause - приостанавливает воспроизведение
- AddSong - добавляет в конец плейлиста песню
- Next воспроизвести след песню
- Prev воспроизвести предыдущую песню
*/

type Player interface {
	// Play - начинает воспроизведение
	Play(ctx context.Context) error
	// Pause - приостанавливает воспроизведение
	Pause(ctx context.Context) error
	// AddSong - добавляет в конец плейлиста песню
	AddSong(ctx context.Context, song Song) error
	// Next воспроизвести след песню
	Next(ctx context.Context) error
	// Prev воспроизвести предыдущую песню
	Prev(ctx context.Context) error
}

type Song struct {
	// Name - название песни
	Name string
	// Duration - длительность песни
	Duration time.Duration
}

type playerNode struct {
	song *Song

	next *playerNode
	prev *playerNode
}

type playerImpl struct {
	mu   sync.RWMutex
	head *playerNode
	tail *playerNode

	current *playerNode

	pauseCh chan struct{}
	stopCh  chan struct{}

	isPlaying  bool
	playedTime time.Duration
}

// Проверяем, что реализация удовлетворяет интерфейсу
var _ Player = &playerImpl{}

// NewPlayer - конструктор для плеера.
func NewPlayer(songs ...Song) (*playerImpl, error) {
	pl := &playerImpl{
		pauseCh: make(chan struct{}),
		stopCh:  make(chan struct{}),
	}

	for i, s := range songs {
		if err := pl.AddSong(context.Background(), s); err != nil {
			return nil, fmt.Errorf("add songs[%d] song: %v", i, err)
		}
	}

	return pl, nil
}

// NewSong - конструктор для Song.
func NewSong(name string, d time.Duration) (Song, error) {
	if name == "" {
		return Song{}, errors.New("song name is empty")
	}

	if d < time.Second {
		return Song{}, errors.New("song duration is less than 1 sec")
	}

	return Song{Name: name, Duration: d}, nil
}

func (p *playerImpl) Play(ctx context.Context) error {
	// плейлист пустой, нечего играть
	// уже воспроизводится песня
	if p.current == nil || p.isPlaying {
		return nil
	}

	if p.playedTime > p.current.song.Duration {
		return p.Next(ctx)
	}

	p.isPlaying = true
	startPlaying := time.Now()
	go func() {
		for {
			select {
			case <-ctx.Done():
				p.isPlaying = false
				p.playedTime = 0
				return
			case <-p.stopCh:
				p.isPlaying = false
				p.playedTime = 0
				return

			case <-p.pauseCh:
				p.isPlaying = false
				p.playedTime += time.Since(startPlaying)
				return

			case <-time.After(p.current.song.Duration - p.playedTime):
				p.playedTime = 0

				// когда достигли конца списка
				// делаем текущую песню первой
				// и останавливаем воспроизведение
				if p.current.next == nil {
					p.isPlaying = false
					p.current = p.head
					return
				}

				p.current = p.current.next
			}
		}
	}()
	return nil
}

func (p *playerImpl) Pause(_ context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.isPlaying {
		return nil
	}

	p.pauseCh <- struct{}{}
	return nil
}

func (p *playerImpl) AddSong(_ context.Context, song Song) error {
	node := &playerNode{song: &song}

	p.mu.Lock()
	defer p.mu.Unlock()

	if p.head == nil {
		p.head, p.tail, p.current = node, node, node
		return nil
	}

	tail := p.tail
	tail.next = node
	node.prev = tail

	p.tail = node
	return nil
}

func (p *playerImpl) Next(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.isPlaying = false
	p.playedTime = 0

	p.stopCh <- struct{}{}
	p.current = p.current.next
	if p.current == nil {
		p.current = p.tail
	}

	return p.Play(ctx)
}

func (p *playerImpl) Prev(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.isPlaying = false
	p.playedTime = 0

	p.stopCh <- struct{}{}
	p.current = p.current.prev
	// если нет предыдущего элемента
	// начинаем воспроизведение с начала.
	if p.current == nil {
		p.current = p.head
	}

	return p.Play(ctx)
}
