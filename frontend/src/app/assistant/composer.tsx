"use client";

import { useEffect, useRef, useState, type FormEvent, type KeyboardEvent } from "react";
import { ArrowUp, Square } from "lucide-react";
import { MAX_TURN_CHARS as MAX_CHARS } from "@/lib/assistant-session";

interface ComposerProps {
  /** Text to start with, such as a prompt typed on the landing page. */
  draft?: string;
  busy: boolean;
  disabled?: boolean;
  disabledReason?: string;
  onSend: (prompt: string) => void;
  onStop: () => void;
}

/** Prompt box: Enter sends, Shift+Enter adds a line, and Stop cancels a run. */
export function Composer({ draft = "", busy, disabled, disabledReason, onSend, onStop }: ComposerProps) {
  const [value, setValue] = useState(draft);
  const ref = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (draft) ref.current?.focus();
  }, [draft]);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
  }, [value]);

  function submit(e?: FormEvent) {
    e?.preventDefault();
    const text = value.trim();
    if (!text || busy || disabled) return;
    onSend(text);
    setValue("");
  }

  function onKeyDown(e: KeyboardEvent<HTMLTextAreaElement>) {
    if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
      e.preventDefault();
      submit();
    }
  }

  const over = value.length > MAX_CHARS;
  return (
    <form
      onSubmit={submit}
      className="sticky bottom-0 z-20 border-t border-border bg-background/95 px-4 py-3 backdrop-blur lg:px-6"
    >
      <div className="flex items-end gap-2 border border-border bg-background px-3 py-2 transition-colors focus-within:border-primary">
        <label htmlFor="assistant-prompt" className="sr-only">
          Ask the assistant
        </label>
        <textarea
          id="assistant-prompt"
          ref={ref}
          rows={1}
          value={value}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={onKeyDown}
          disabled={disabled}
          placeholder={disabled ? disabledReason : "Describe a mood, a plot, or a title you loved"}
          className="max-h-40 min-h-6 flex-1 resize-none bg-transparent py-1 font-serif text-base text-foreground placeholder:text-muted-foreground focus:outline-none disabled:cursor-not-allowed"
        />
        {busy ? (
          <button
            type="button"
            onClick={onStop}
            aria-label="Stop"
            className="flex size-9 shrink-0 items-center justify-center border border-foreground text-foreground transition-colors hover:bg-foreground hover:text-background"
          >
            <Square className="size-3.5" fill="currentColor" />
          </button>
        ) : (
          <button
            type="submit"
            aria-label="Send"
            disabled={!value.trim() || over || disabled}
            className="flex size-9 shrink-0 items-center justify-center bg-primary text-primary-foreground transition-colors hover:bg-primary/90 disabled:bg-muted disabled:text-muted-foreground"
          >
            <ArrowUp className="size-4" />
          </button>
        )}
      </div>
      <div className="mt-1.5 flex justify-between font-mono text-[10px] text-muted-foreground">
        <span>Enter to send · Shift+Enter for a new line</span>
        {value.length > MAX_CHARS - 150 && (
          <span className={over ? "text-destructive" : undefined}>
            {value.length}/{MAX_CHARS}
          </span>
        )}
      </div>
    </form>
  );
}
