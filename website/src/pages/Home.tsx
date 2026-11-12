import { Architecture } from "../components/sections/Architecture";
import { CLIShowcase } from "../components/sections/CLIShowcase";
import { CodeShowcase } from "../components/sections/CodeShowcase";
import { Community } from "../components/sections/Community";
import { Documentation } from "../components/sections/Documentation";
import { Downloads } from "../components/sections/Downloads";
import { Features } from "../components/sections/Features";
import { Hero } from "../components/sections/Hero";
import { Introduction } from "../components/sections/Introduction";
import { Roadmap } from "../components/sections/Roadmap";

export function Home() {
  return (
    <>
      <Hero />
      <Introduction />
      <Features />
      <CodeShowcase />
      <Architecture />
      <CLIShowcase />
      <Downloads />
      <Documentation />
      <Roadmap />
      <Community />
    </>
  );
}
