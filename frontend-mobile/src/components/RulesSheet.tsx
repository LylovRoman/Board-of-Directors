export function RulesSheetContent() {
  return (
    <div className="rules-list">
      <article>
        <strong>1. Роли</strong>
        <p>Один игрок тайно становится Кротом. Остальные играют за совет директоров.</p>
      </article>
      <article>
        <strong>2. Major vote</strong>
        <p>Голоса считаются по долям. Каждый выбирает одну карточку из текущей витрины.</p>
      </article>
      <article>
        <strong>3. Цели Крота</strong>
        <p>Крот выбирает три Подкопа и одну Диверсию. Подкоп дает 1 очко, Диверсия дает 2.</p>
      </article>
      <article>
        <strong>4. Governance</strong>
        <p>После принятого решения игроки могут менять доли через передачу, грант из резерва или выкуп.</p>
      </article>
      <article>
        <strong>5. Победа</strong>
        <p>Крот выигрывает при 3 очках саботажа. Совет выигрывает при 3 чистых решениях.</p>
      </article>
    </div>
  );
}
