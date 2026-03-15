"use client";

import { useRef, useEffect } from "react";

const VERTEX_SHADER = `
attribute vec2 a_position;
attribute vec2 a_texCoord;
varying vec2 v_texCoord;

void main() {
  gl_Position = vec4(a_position, 0.0, 1.0);
  v_texCoord = a_texCoord;
}
`;

const FRAGMENT_SHADER = `
precision mediump float;

uniform sampler2D u_image;
uniform vec3 u_keyColor;
uniform float u_similarity;
uniform float u_smoothness;
uniform float u_spill;

varying vec2 v_texCoord;

// RGB to YCbCr - separates luminance from chrominance for better keying
vec2 rgbToUV(vec3 rgb) {
  return vec2(
    rgb.r * -0.169 + rgb.g * -0.331 + rgb.b *  0.5   + 0.5,
    rgb.r *  0.5   + rgb.g * -0.419 + rgb.b * -0.081 + 0.5
  );
}

void main() {
  vec4 color = texture2D(u_image, v_texCoord);

  vec2 uvKey   = rgbToUV(u_keyColor);
  vec2 uvPixel = rgbToUV(color.rgb);

  float dist  = distance(uvKey, uvPixel);
  float alpha = smoothstep(u_similarity, u_similarity + u_smoothness, dist);

  // Spill removal: suppress green reflected onto edges
  float spillVal = smoothstep(u_similarity, u_similarity + u_smoothness * 2.0, dist);
  color.rgb = mix(color.rgb - u_spill * vec3(0.0, 1.0, 0.0), color.rgb, spillVal);
  color.rgb = max(color.rgb, 0.0);

  gl_FragColor = vec4(color.rgb * alpha, alpha);
}
`;

interface ChromaKeyVideoProps {
  src: string;
  className?: string;
  /** Key color to remove as [r, g, b] normalized 0-1. Default: [0.0, 1.0, 0.0] */
  keyColor?: [number, number, number];
  /** Similarity threshold (0-1). Lower = stricter match. Default: 0.2 */
  similarity?: number;
  /** Edge smoothness (0-1). Default: 0.08 */
  smoothness?: number;
  /** Spill suppression strength (0-1). Default: 0.1 */
  spill?: number;
  /** Called once when video is ready and WebGL is initialized */
  onCanPlay?: () => void;
}

function compileShader(gl: WebGLRenderingContext, type: number, source: string): WebGLShader {
  const shader = gl.createShader(type)!;
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    const info = gl.getShaderInfoLog(shader);
    gl.deleteShader(shader);
    throw new Error(`Shader compile error: ${info}`);
  }
  return shader;
}

export default function ChromaKeyVideo({
  src,
  className,
  keyColor = [0.0, 1.0, 0.0],
  similarity = 0.2,
  smoothness = 0.08,
  spill = 0.1,
  onCanPlay,
}: ChromaKeyVideoProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const videoRef = useRef<HTMLVideoElement>(null);
  const rafRef = useRef<number>(0);
  const glRef = useRef<{
    gl: WebGLRenderingContext;
    texture: WebGLTexture;
    uniforms: Record<string, WebGLUniformLocation>;
  } | null>(null);

  useEffect(() => {
    const video = videoRef.current;
    const canvas = canvasRef.current;
    if (!video || !canvas) return;

    let running = true;

    function initGL() {
      if (glRef.current) return; // already initialized

      const gl = canvas!.getContext("webgl", {
        premultipliedAlpha: true,
        alpha: true,
      });
      if (!gl) return;

      const vs = compileShader(gl, gl.VERTEX_SHADER, VERTEX_SHADER);
      const fs = compileShader(gl, gl.FRAGMENT_SHADER, FRAGMENT_SHADER);
      const program = gl.createProgram()!;
      gl.attachShader(program, vs);
      gl.attachShader(program, fs);
      gl.linkProgram(program);
      gl.useProgram(program);

      // Full-screen quad
      const positions = new Float32Array([
        -1, -1, 1, -1, -1, 1,
        -1, 1, 1, -1, 1, 1,
      ]);
      const texCoords = new Float32Array([
        0, 1, 1, 1, 0, 0,
        0, 0, 1, 1, 1, 0,
      ]);

      const posBuf = gl.createBuffer()!;
      gl.bindBuffer(gl.ARRAY_BUFFER, posBuf);
      gl.bufferData(gl.ARRAY_BUFFER, positions, gl.STATIC_DRAW);
      const aPos = gl.getAttribLocation(program, "a_position");
      gl.enableVertexAttribArray(aPos);
      gl.vertexAttribPointer(aPos, 2, gl.FLOAT, false, 0, 0);

      const texBuf = gl.createBuffer()!;
      gl.bindBuffer(gl.ARRAY_BUFFER, texBuf);
      gl.bufferData(gl.ARRAY_BUFFER, texCoords, gl.STATIC_DRAW);
      const aTex = gl.getAttribLocation(program, "a_texCoord");
      gl.enableVertexAttribArray(aTex);
      gl.vertexAttribPointer(aTex, 2, gl.FLOAT, false, 0, 0);

      const texture = gl.createTexture()!;
      gl.bindTexture(gl.TEXTURE_2D, texture);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR);
      gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR);

      gl.enable(gl.BLEND);
      gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA);

      const uniforms = {
        u_image: gl.getUniformLocation(program, "u_image")!,
        u_keyColor: gl.getUniformLocation(program, "u_keyColor")!,
        u_similarity: gl.getUniformLocation(program, "u_similarity")!,
        u_smoothness: gl.getUniformLocation(program, "u_smoothness")!,
        u_spill: gl.getUniformLocation(program, "u_spill")!,
      };

      gl.uniform1i(uniforms.u_image, 0);
      glRef.current = { gl, texture, uniforms };
    }

    function renderFrame() {
      if (!running) return;

      const ctx = glRef.current;
      if (ctx && video!.readyState >= 2 && !video!.paused) {
        const { gl, texture, uniforms } = ctx;

        gl.activeTexture(gl.TEXTURE0);
        gl.bindTexture(gl.TEXTURE_2D, texture);
        gl.texImage2D(gl.TEXTURE_2D, 0, gl.RGBA, gl.RGBA, gl.UNSIGNED_BYTE, video!);

        gl.uniform3f(uniforms.u_keyColor, keyColor[0], keyColor[1], keyColor[2]);
        gl.uniform1f(uniforms.u_similarity, similarity);
        gl.uniform1f(uniforms.u_smoothness, smoothness);
        gl.uniform1f(uniforms.u_spill, spill);

        gl.viewport(0, 0, gl.canvas.width, gl.canvas.height);
        gl.clearColor(0, 0, 0, 0);
        gl.clear(gl.COLOR_BUFFER_BIT);
        gl.drawArrays(gl.TRIANGLES, 0, 6);
      }

      rafRef.current = requestAnimationFrame(renderFrame);
    }

    function start() {
      canvas!.width = video!.videoWidth || 1280;
      canvas!.height = video!.videoHeight || 720;
      initGL();
      rafRef.current = requestAnimationFrame(renderFrame);
      onCanPlay?.();
    }

    // Handle both: video already loaded (readyState >= 2) OR not yet loaded
    if (video.readyState >= 2) {
      start();
    } else {
      video.addEventListener("loadeddata", start, { once: true });
    }

    // Also try to play in case autoplay was blocked
    video.play().catch(() => {});

    return () => {
      running = false;
      cancelAnimationFrame(rafRef.current);
      video.removeEventListener("loadeddata", start);
    };
  }, [keyColor, similarity, smoothness, spill]);

  return (
    <div className={className}>
      <video
        ref={videoRef}
        src={src}
        autoPlay
        loop
        muted
        playsInline
        className="absolute left-0 top-0 size-0 opacity-0"
      />
      <canvas ref={canvasRef} className="size-full" />
    </div>
  );
}
