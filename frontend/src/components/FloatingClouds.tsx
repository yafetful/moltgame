"use client";

/**
 * Clouds drift continuously from left to right across the screen.
 * When a cloud exits the right edge, it seamlessly reappears from the left.
 *
 * Positions (top/width) from Figma 1920x1080 canvas, converted to percentages.
 * Each cloud's initial Figma X position is mapped to a negative animation-delay
 * so it starts at the correct horizontal position on first render.
 */

const CLOUDS = [
  { src: "/images/cloud1.svg", top: 29.9, left:  3.3, w: 10.4, duration: 20 },
  { src: "/images/cloud2.svg", top:  9.2, left: 13.8, w: 18.8, duration: 28 },
  { src: "/images/cloud3.svg", top: 21.4, left: 77.0, w: 20.8, duration: 24 },
  { src: "/images/cloud4.svg", top: 23.1, left: 19.6, w: 29.2, duration: 32 },
  { src: "/images/cloud5.svg", top: 11.9, left: 47.7, w: 37.5, duration: 36 },
  { src: "/images/cloud6.svg", top: 32.1, left: 42.4, w: 20.8, duration: 22 },
];

export default function FloatingClouds() {
  return (
    <>
      <style>{`
        @keyframes cloud-drift {
          from { transform: translateX(0); }
          to   { transform: translateX(calc(100vw + 100%)); }
        }
      `}</style>
      <div className="pointer-events-none absolute inset-0 z-0 overflow-hidden">
        {CLOUDS.map((cloud, i) => {
          // Calculate negative delay so the cloud starts at its Figma X position.
          // Animation travels from left:-{w}vw to left:100vw, total = (100+w)vw.
          // Fraction into cycle = (figmaLeft + w) / (100 + w)
          const fraction = (cloud.left + cloud.w) / (100 + cloud.w);
          const delay = -(fraction * cloud.duration);

          return (
            <img
              key={i}
              src={cloud.src}
              alt=""
              aria-hidden
              className="absolute opacity-60"
              style={{
                top: `${cloud.top}%`,
                left: `-${cloud.w}vw`,
                width: `${cloud.w}vw`,
                animation: `cloud-drift ${cloud.duration}s linear ${delay}s infinite`,
              }}
            />
          );
        })}
      </div>
    </>
  );
}
