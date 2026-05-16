// Step4Persona — Persona profile definition step.
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2 Step 4
import { useState } from "react";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Textarea } from "@/components/ui/textarea";
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group";
import type { HonorificLevel, PersonaProfile } from "@/types/onboarding";
import type { StepProps } from "./StepProps";

// Characters forbidden in persona names to prevent injection.
const FORBIDDEN_CHARS = /[<>;&#]/;

function validateName(name: string): string | null {
  const trimmed = name.trim();
  if (trimmed.length === 0) return "이름을 입력해 주세요. / Name is required.";
  if (FORBIDDEN_CHARS.test(trimmed))
    return "이름에 <, >, ;, &, # 문자를 사용할 수 없습니다. / Name contains forbidden characters.";
  if (trimmed.length > 500)
    return "이름이 너무 깁니다. / Name is too long (max 500).";
  return null;
}

const HONORIFIC_OPTIONS: { value: HonorificLevel; label: string }[] = [
  { value: "formal", label: "격식체 / Formal" },
  { value: "casual", label: "반말 / Casual" },
  { value: "intimate", label: "친근체 / Intimate" },
];

export function Step4Persona({
  data,
  loading,
  submitStep,
  back,
  canBack,
}: StepProps) {
  const persona = data.Persona;
  const [name, setName] = useState(persona.Name || "");
  const [honorific, setHonorific] = useState<HonorificLevel>(
    persona.HonorificLevel || "formal"
  );
  const [pronouns, setPronouns] = useState(persona.Pronouns || "");
  const [soul, setSoul] = useState(persona.SoulMarkdown || "");
  const [nameError, setNameError] = useState<string | null>(null);
  const [localError, setLocalError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  function handleNameChange(value: string) {
    setName(value);
    setNameError(validateName(value));
  }

  const isNameValid = validateName(name) === null;
  const isBusy = loading || pending;

  async function handleSubmit() {
    const err = validateName(name);
    if (err != null) {
      setNameError(err);
      return;
    }
    setPending(true);
    setLocalError(null);
    try {
      const body: PersonaProfile = {
        Name: name.trim(),
        HonorificLevel: honorific,
        Pronouns: pronouns.trim(),
        SoulMarkdown: soul,
      };
      await submitStep<PersonaProfile>(4, body);
    } catch (e) {
      setLocalError(e instanceof Error ? e.message : "오류가 발생했습니다.");
    } finally {
      setPending(false);
    }
  }

  return (
    <Card className="w-full max-w-lg mx-auto">
      <CardHeader>
        <CardTitle>Step 4: 페르소나 / Persona</CardTitle>
        <CardDescription>
          MINK의 이름과 대화 스타일을 설정하세요.
          <br />
          Set MINK's name and conversation style.
        </CardDescription>
      </CardHeader>

      <CardContent className="space-y-5">
        {/* Name (required) */}
        <div className="space-y-2">
          <Label htmlFor="persona-name">
            이름 / Name <span className="text-destructive">*</span>
          </Label>
          <Input
            id="persona-name"
            placeholder="MINK"
            value={name}
            maxLength={500}
            onChange={(e) => handleNameChange(e.target.value)}
            disabled={isBusy}
            aria-invalid={nameError != null}
            aria-describedby={nameError != null ? "name-error" : undefined}
            className={nameError != null ? "border-destructive ring-destructive" : ""}
          />
          {nameError != null && (
            <p id="name-error" className="text-xs text-destructive">
              {nameError}
            </p>
          )}
        </div>

        {/* HonorificLevel (required radio) */}
        <div className="space-y-2">
          <Label>경어 수준 / Honorific Level</Label>
          <RadioGroup
            value={honorific}
            onValueChange={(v) => setHonorific(v as HonorificLevel)}
            disabled={isBusy}
            className="flex gap-4"
          >
            {HONORIFIC_OPTIONS.map((opt) => (
              <div key={opt.value} className="flex items-center gap-2">
                <RadioGroupItem
                  value={opt.value}
                  id={`honorific-${opt.value}`}
                />
                <Label htmlFor={`honorific-${opt.value}`} className="cursor-pointer">
                  {opt.label}
                </Label>
              </div>
            ))}
          </RadioGroup>
        </div>

        {/* Pronouns (optional) */}
        <div className="space-y-2">
          <Label htmlFor="persona-pronouns">
            대명사 / Pronouns{" "}
            <span className="text-muted-foreground font-normal text-xs">(선택 / optional)</span>
          </Label>
          <Input
            id="persona-pronouns"
            placeholder="그/그녀/they..."
            value={pronouns}
            onChange={(e) => setPronouns(e.target.value)}
            disabled={isBusy}
          />
        </div>

        {/* SoulMarkdown (optional) */}
        <div className="space-y-2">
          <Label htmlFor="persona-soul">
            소울 마크다운 / Soul Markdown{" "}
            <span className="text-muted-foreground font-normal text-xs">(선택 / optional)</span>
          </Label>
          <Textarea
            id="persona-soul"
            rows={8}
            placeholder="MINK persona's soul markdown — Phase 2 NLG context"
            value={soul}
            onChange={(e) => setSoul(e.target.value)}
            disabled={isBusy}
          />
          <p className="text-xs text-muted-foreground">
            페르소나의 성격, 말투, 배경을 자유롭게 서술하세요.
            <br />
            Describe the persona's personality, tone, and background freely.
          </p>
        </div>

        {localError != null && (
          <p className="text-xs text-destructive bg-destructive/10 border border-destructive/20 rounded px-3 py-2">
            {localError}
          </p>
        )}
      </CardContent>

      <CardFooter className="flex justify-between gap-2">
        <Button
          variant="outline"
          onClick={back}
          disabled={!canBack || isBusy}
        >
          뒤로 / Back
        </Button>
        <Button onClick={handleSubmit} disabled={!isNameValid || isBusy}>
          {isBusy ? "처리 중..." : "Continue / 다음"}
        </Button>
      </CardFooter>
    </Card>
  );
}
