import { useState } from "react";

export function TutorialDialog(props: { onClose: () => void }) {
  const steps = [
    { title: "1. Осмотрись в комнате", text: "Название комнаты показывает, куда ты вошел, а карточка компании задает сюжет партии. До старта Host может пригласить игроков или добавить ботов." },
    { title: "2. Получи роль", text: "Директору нужно провести чистые решения, Кроту - замаскировать 3 Подкопа и 1 Диверсию, а Комплаенсу - тайно следить за подозреваемыми до принятой Диверсии." },
    { title: "3. Выбери меморандум", text: "Обычный Директор выбирает стартовую подсказку про возможности или риски. Комплаенс выбирает только предпочтение: стартовый меморандум он не получает, зато этот тип используется для поздней подсказки после Диверсии, если партия продолжается." },
    { title: "4. Голосуй в major vote", text: "Выбирай решение из витрины из 4 карточек и меняй голос, пока раунд открыт. Комплаенс кликом выбирает цель наблюдения и может переставить ее до закрытия major vote." },
    { title: "5. Разбирай governance", text: "После принятого решения можно менять доли, выдавать гранты или делать выкуп. Здесь сила голоса равна доле плюс полномочия." },
    { title: "6. Следи за таймером", text: "На фазу есть 3 минуты. Если игрок не делает обязательный ход, его место занимает бот-заместитель; обычные боты ждут 10 секунд перед автоходом." },
    { title: "7. Читай финал", text: "В финале смотри победителя, цели Крота, Комплаенса, точность major-голосов, XP и replay. За победную поимку Крота Комплаенс получает отдельный бонус +25 XP." },
    { title: "8. Усиленный меморандум", text: "Обычные Директора получают прежнюю тройку. Комплаенс после принятой Диверсии, если партия продолжается, получает усиленную пару выбранного типа: в рисках есть цель Крота, в возможностях есть чистое решение." },
  ];
  const [index, setIndex] = useState(0);
  const step = steps[index];
  return (
      <div className="modal-backdrop" role="presentation">
        <section className="tutorial-dialog" role="dialog" aria-modal="true" aria-labelledby="tutorial-title">
          <div className="profile-dialog-header">
            <div>
              <p className="eyebrow">обучение</p>
              <h2 id="tutorial-title">Быстрый ввод в партию</h2>
            </div>
            <button className="mini-button" onClick={props.onClose}>Закрыть</button>
          </div>
          <div className="tutorial-card">
            <h3>{step.title}</h3>
            <p>{step.text}</p>
          </div>
          <div className="tutorial-track">
            {steps.map((item, itemIndex) => (
                <button key={item.title} className={itemIndex === index ? "tutorial-dot active" : "tutorial-dot"} onClick={() => setIndex(itemIndex)} aria-label={item.title} />
            ))}
          </div>
          <div className="toolbar-actions centered-actions">
            <button className="secondary-action" onClick={() => setIndex((value) => Math.max(0, value - 1))} disabled={index === 0}>Назад</button>
            <button className="primary-action" onClick={() => setIndex((value) => Math.min(steps.length - 1, value + 1))} disabled={index === steps.length - 1}>Дальше</button>
          </div>
        </section>
      </div>
  );
}
