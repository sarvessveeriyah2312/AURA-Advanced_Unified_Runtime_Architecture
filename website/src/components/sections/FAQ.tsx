import { FAQS } from "../../data/faq";
import { Reveal } from "../ui/Reveal";
import { Section, SectionHeading } from "../ui/SectionHeading";

// Frequently asked questions. Every answer is visible text in the page
// (not collapsed), so readers, search engines, and AI assistants can all
// read it; the same list is emitted as FAQPage structured data.
export function FAQ() {
  return (
    <Section id="faq" labelledBy="faq-title">
      <SectionHeading
        id="faq-title"
        eyebrow="FAQ"
        title="Questions developers ask about Tread."
        description="Short, direct answers about what Tread is, who it is for, and what it can do today."
      />
      <div className="mt-12 grid grid-cols-1 gap-x-10 gap-y-8 md:grid-cols-2">
        {FAQS.map((f, i) => (
          <Reveal key={f.question} delay={0.03 * (i % 2)} className="border-t border-line pt-5">
            <h3 className="font-semibold">{f.question}</h3>
            <p className="mt-2 text-[15px] leading-relaxed text-muted">{f.answer}</p>
          </Reveal>
        ))}
      </div>
    </Section>
  );
}
