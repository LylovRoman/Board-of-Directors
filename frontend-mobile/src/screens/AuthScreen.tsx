import { LogIn, UserPlus } from "lucide-react";
import type { FormEvent } from "react";

type AuthMode = "login" | "register";

interface AuthScreenProps {
  mode: AuthMode;
  login: string;
  password: string;
  name: string;
  avatarUrl: string;
  isSubmitting: boolean;
  onModeChange: (mode: AuthMode) => void;
  onLoginChange: (value: string) => void;
  onPasswordChange: (value: string) => void;
  onNameChange: (value: string) => void;
  onAvatarUrlChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
}

export function AuthScreen(props: AuthScreenProps) {
  const isRegister = props.mode === "register";

  return (
    <main className="mobile-shell auth-shell">
      <section className="auth-card">
        <p className="eyebrow">Board of Directors</p>
        <h1>Мобильный совет директоров</h1>
        <p className="auth-copy">
          Входи в комнату, голосуй, веди переговоры и следи за корпоративными маневрами прямо с телефона.
        </p>

        <div className="segmented-control" role="tablist" aria-label="Режим авторизации">
          <button
            type="button"
            className={props.mode === "login" ? "active" : ""}
            onClick={() => props.onModeChange("login")}
          >
            Вход
          </button>
          <button
            type="button"
            className={isRegister ? "active" : ""}
            onClick={() => props.onModeChange("register")}
          >
            Регистрация
          </button>
        </div>

        <form className="stack-form" onSubmit={props.onSubmit}>
          <label>
            <span>Логин</span>
            <input
              autoComplete="username"
              value={props.login}
              onChange={(event) => props.onLoginChange(event.target.value)}
              placeholder="director"
            />
          </label>
          <label>
            <span>Пароль</span>
            <input
              autoComplete={isRegister ? "new-password" : "current-password"}
              type="password"
              value={props.password}
              onChange={(event) => props.onPasswordChange(event.target.value)}
              placeholder="••••••••"
            />
          </label>
          {isRegister ? (
            <>
              <label>
                <span>Имя в игре</span>
                <input
                  autoComplete="name"
                  value={props.name}
                  onChange={(event) => props.onNameChange(event.target.value)}
                  placeholder="Алиса"
                />
              </label>
              <label>
                <span>Аватар URL</span>
                <input
                  inputMode="url"
                  value={props.avatarUrl}
                  onChange={(event) => props.onAvatarUrlChange(event.target.value)}
                  placeholder="https://..."
                />
              </label>
            </>
          ) : null}
          <button className="primary-action wide-action" type="submit" disabled={props.isSubmitting}>
            {isRegister ? <UserPlus size={18} /> : <LogIn size={18} />}
            {isRegister ? "Создать профиль" : "Войти"}
          </button>
        </form>
      </section>
    </main>
  );
}
