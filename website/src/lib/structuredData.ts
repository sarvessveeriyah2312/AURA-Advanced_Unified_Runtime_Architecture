import { FAQS } from "../data/faq";
import { AUTHOR, LICENSE_URL, MOTTO, REPO_URL, SITE_URL } from "../data/site";

const DESCRIPTION =
  "Tread is a small, statically typed programming language built to teach how languages work, with a readable lexer, parser, type checker, bytecode compiler, and virtual machine written in Go.";

// JSON-LD for the homepage, injected into the prerendered HTML at build
// time (scripts/prerender.mjs). It only restates facts that are also
// visible on the page, as structured data must.
export function structuredData() {
  const author = { "@type": "Person", "@id": `${SITE_URL}#author`, name: AUTHOR.name, url: AUTHOR.url };
  return {
    "@context": "https://schema.org",
    "@graph": [
      {
        "@type": "WebSite",
        "@id": `${SITE_URL}#website`,
        url: SITE_URL,
        name: "Tread",
        description: `${MOTTO} ${DESCRIPTION}`,
        inLanguage: "en",
        author: { "@id": `${SITE_URL}#author` },
      },
      {
        "@type": "ComputerLanguage",
        "@id": `${SITE_URL}#language`,
        name: "Tread",
        description: DESCRIPTION,
        url: SITE_URL,
        sameAs: [REPO_URL],
      },
      {
        "@type": "SoftwareSourceCode",
        "@id": `${SITE_URL}#source`,
        name: "Tread",
        description: DESCRIPTION,
        codeRepository: REPO_URL,
        programmingLanguage: "Go",
        license: LICENSE_URL,
        author: { "@id": `${SITE_URL}#author` },
      },
      author,
      {
        "@type": "FAQPage",
        "@id": `${SITE_URL}#faq`,
        mainEntity: FAQS.map((f) => ({
          "@type": "Question",
          name: f.question,
          acceptedAnswer: { "@type": "Answer", text: f.answer },
        })),
      },
    ],
  };
}
