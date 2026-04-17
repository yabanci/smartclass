import { Sparkles } from 'lucide-react';

export function SplashScreen() {
  return (
    <div
      className="w-full h-full flex items-center justify-center"
      style={{ background: 'linear-gradient(160deg, #1E3A8A 0%, #0f2560 40%, #06B6D4 100%)' }}
    >
      <div className="text-center text-white">
        <div className="w-24 h-24 rounded-3xl bg-white/20 flex items-center justify-center mx-auto mb-6 animate-glow">
          <Sparkles size={48} />
        </div>
        <h1 className="text-2xl font-bold">Smart Classroom</h1>
        <p className="text-sm opacity-70 mt-2">Loading...</p>
      </div>
    </div>
  );
}
