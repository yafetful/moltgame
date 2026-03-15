"use client";

import { useState } from "react";
import ChromaKeyVideo from "./ChromaKeyVideo";

export default function HeroSection() {
  const [canPlay, setCanPlay] = useState(false);

  return (
    <div className="pointer-events-none absolute inset-x-0 bottom-0 z-20">
      {/* Fallback static images — mobile only, hidden once video plays */}
      {!canPlay && (
        <div className="absolute inset-x-0 bottom-0 h-[50vh] md:hidden">
          {/* leftmost character */}
          <img
            src="/images/hero-char3.png"
            alt=""
            className="absolute bottom-0 left-0 w-[45%] object-contain object-bottom"
          />
          {/* center character */}
          <img
            src="/images/hero-char1.png"
            alt=""
            className="absolute bottom-[28%] left-[35%] w-[39%] object-contain object-bottom"
          />
          {/* rightmost character */}
          <img
            src="/images/hero-char2.png"
            alt=""
            className="absolute bottom-0 left-[64%] w-[35%] object-contain object-bottom"
          />
        </div>
      )}

      {/* Video with chroma key */}
      <div className="mx-auto flex w-full max-w-4xl justify-center origin-bottom scale-110 md:scale-100 md:origin-center">
        <ChromaKeyVideo
          src="/hero-animation.mp4"
          className="aspect-video w-full"
          keyColor={[0.0, 1.0, 0.0]}
          similarity={0.3}
          smoothness={0.15}
          spill={0.3}
          onCanPlay={() => setCanPlay(true)}
        />
      </div>
    </div>
  );
}
