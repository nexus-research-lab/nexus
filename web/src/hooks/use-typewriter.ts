import { useEffect, useState, useRef } from 'react';

interface UseTypewriterProps {
    text: string;
    speed?: number; // ms per character
    enabled?: boolean;
    onComplete?: () => void;
}

export function useTypewriter({
    text,
    speed = 10,
    enabled = true,
    onComplete
}: UseTypewriterProps) {
    const [displayedText, setDisplayedText] = useState('');
    const [isComplete, setIsComplete] = useState(false);

    // Use refs to keep track of current state without triggering re-renders
    const indexRef = useRef(0);
    const textRef = useRef(text);
    const animationFrameRef = useRef<number | null>(null);
    const lastUpdateRef = useRef<number>(0);
    const onCompleteRef = useRef(onComplete);

    // Update refs when props change
    useEffect(() => {
        textRef.current = text;
        onCompleteRef.current = onComplete;
    }, [text, onComplete]);

    useEffect(() => {
        if (!enabled) {
            setDisplayedText(text);
            setIsComplete(true);
            if (onCompleteRef.current) onCompleteRef.current();
            return;
        }

        // Cancel any existing animation
        if (animationFrameRef.current) {
            cancelAnimationFrame(animationFrameRef.current);
            animationFrameRef.current = null;
        }

        // Reset if text completely changed (e.g., new message)
        if (indexRef.current > text.length) {
            indexRef.current = 0;
            setDisplayedText('');
            setIsComplete(false);
        }

        const animate = (timestamp: number) => {
            const currentText = textRef.current;
            const currentIndex = indexRef.current;

            // Throttle updates based on speed
            if (timestamp - lastUpdateRef.current < speed) {
                animationFrameRef.current = requestAnimationFrame(animate);
                return;
            }

            lastUpdateRef.current = timestamp;

            if (currentIndex < currentText.length) {
                // Adaptive chunk size: faster as we catch up
                const remaining = currentText.length - currentIndex;
                const chunkSize = Math.max(1, Math.min(3, Math.ceil(remaining / 20)));

                const nextIndex = Math.min(currentIndex + chunkSize, currentText.length);
                setDisplayedText(currentText.slice(0, nextIndex));
                indexRef.current = nextIndex;

                animationFrameRef.current = requestAnimationFrame(animate);
            } else if (currentIndex === currentText.length && !isComplete) {
                setIsComplete(true);
                if (onCompleteRef.current) onCompleteRef.current();
            }
        };

        // Start animation
        animationFrameRef.current = requestAnimationFrame(animate);

        return () => {
            if (animationFrameRef.current) {
                cancelAnimationFrame(animationFrameRef.current);
                animationFrameRef.current = null;
            }
        };
    }, [text, enabled, speed, isComplete]);

    return {
        displayedText,
        isComplete
    };
}
