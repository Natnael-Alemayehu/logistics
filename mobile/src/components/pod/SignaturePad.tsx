import React, { useState, useRef, useCallback, useEffect, forwardRef, useImperativeHandle } from 'react';
import {
  View,
  StyleSheet,
  TouchableOpacity,
  Text,
  Dimensions,
} from 'react-native';
import {
  Canvas,
  Path,
  Skia,
  useCanvasRef,
  Group,
  rect,
  Paint,
  Color,
} from '@shopify/react-native-skia';
import {
  Gesture,
  GestureDetector,
} from 'react-native-gesture-handler';
import { colors, spacing } from '../../utils/theme';

const { width } = Dimensions.get('window');
const CANVAS_WIDTH = width - spacing.xl * 2;
const CANVAS_HEIGHT = 200;

interface Point {
  x: number;
  y: number;
}

interface Stroke {
  points: Point[];
}

interface SignaturePadProps {
  onSignatureChange: (base64: string | null) => void;
  editable?: boolean;
}

export interface SignaturePadRef {
  exportSignature: () => Promise<string | null>;
  clearSignature: () => void;
  hasSignature: () => boolean;
}

const SignaturePad = forwardRef<SignaturePadRef, SignaturePadProps>(
  ({ onSignatureChange, editable = true }, ref) => {
    const [strokes, setStrokes] = useState<Stroke[]>([]);
    const [currentStroke, setCurrentStroke] = useState<Point[]>([]);
    const canvasRef = useCanvasRef();
    const lastPointRef = useRef<Point | null>(null);

    const hasSignatureData = strokes.length > 0;

    useImperativeHandle(ref, () => ({
      exportSignature: async (): Promise<string | null> => {
        if (strokes.length === 0) return null;

        try {
          if (!canvasRef.current) return null;

          const snapshot = canvasRef.current.makeImageSnapshot();
          if (!snapshot) return null;

          const base64 = snapshot.encodeToBase64();
          return base64;
        } catch (error) {
          console.error('Failed to export signature:', error);
          return null;
        }
      },
      clearSignature: () => {
        setStrokes([]);
        setCurrentStroke([]);
        onSignatureChange(null);
      },
      hasSignature: () => strokes.length > 0,
    }));

    useEffect(() => {
      if (!hasSignatureData) {
        onSignatureChange(null);
      }
    }, [hasSignatureData, onSignatureChange]);

    const addPoint = useCallback((x: number, y: number) => {
      const clampedX = Math.max(0, Math.min(CANVAS_WIDTH, x));
      const clampedY = Math.max(0, Math.min(CANVAS_HEIGHT, y));

      setCurrentStroke((prev) => {
        const newPoints = [...prev, { x: clampedX, y: clampedY }];
        return newPoints;
      });
    }, []);

    const startStroke = useCallback((x: number, y: number) => {
      if (!editable) return;
      lastPointRef.current = { x, y };
      setCurrentStroke([{ x, y }]);
    }, [editable]);

    const continueStroke = useCallback((x: number, y: number) => {
      if (!editable) return;
      addPoint(x, y);
    }, [editable, addPoint]);

    const endStroke = useCallback(() => {
      if (!editable) return;

      if (currentStroke.length > 1) {
        setStrokes((prev) => [...prev, { points: currentStroke }]);
      }
      setCurrentStroke([]);
      lastPointRef.current = null;
    }, [currentStroke, editable]);

    const panGesture = Gesture.Pan()
      .onStart((e) => {
        startStroke(e.x, e.y);
      })
      .onUpdate((e) => {
        continueStroke(e.x, e.y);
      })
      .onEnd(() => {
        endStroke();
      })
      .minDistance(0)
      .minPointers(1)
      .maxPointers(1);

    const clearSignature = useCallback(() => {
      setStrokes([]);
      setCurrentStroke([]);
      onSignatureChange(null);
    }, [onSignatureChange]);

    const renderStroke = (stroke: Stroke, index: number) => {
      if (stroke.points.length < 2) return null;

      const path = Skia.Path.Make();
      path.moveTo(stroke.points[0].x, stroke.points[0].y);

      for (let i = 1; i < stroke.points.length; i++) {
        const prevPoint = stroke.points[i - 1];
        const currentPoint = stroke.points[i];
        const midX = (prevPoint.x + currentPoint.x) / 2;
        const midY = (prevPoint.y + currentPoint.y) / 2;
        path.quadTo(prevPoint.x, prevPoint.y, midX, midY);
      }

      const lastPoint = stroke.points[stroke.points.length - 1];
      path.lineTo(lastPoint.x, lastPoint.y);

      return (
        <Path
          key={index}
          path={path}
          style="stroke"
          color={colors.text}
          strokeWidth={2}
          strokeCap="round"
          strokeJoin="round"
        />
      );
    };

    const renderCurrentStroke = () => {
      if (currentStroke.length < 2) return null;

      const path = Skia.Path.Make();
      path.moveTo(currentStroke[0].x, currentStroke[0].y);

      for (let i = 1; i < currentStroke.length; i++) {
        const prevPoint = currentStroke[i - 1];
        const currentPoint = currentStroke[i];
        const midX = (prevPoint.x + currentPoint.x) / 2;
        const midY = (prevPoint.y + currentPoint.y) / 2;
        path.quadTo(prevPoint.x, prevPoint.y, midX, midY);
      }

      const lastPoint = currentStroke[currentStroke.length - 1];
      path.lineTo(lastPoint.x, lastPoint.y);

      return (
        <Path
          path={path}
          style="stroke"
          color={colors.text}
          strokeWidth={2}
          strokeCap="round"
          strokeJoin="round"
        />
      );
    };

    const hasSignature = strokes.length > 0 || currentStroke.length > 0;

    return (
      <View style={styles.container}>
        <View style={[styles.canvasContainer, !editable && styles.disabled]}>
          <GestureDetector gesture={panGesture}>
            <Canvas
              ref={canvasRef}
              style={styles.canvas}
            >
              <Group>
                <rect
                  x={0}
                  y={0}
                  width={CANVAS_WIDTH}
                  height={CANVAS_HEIGHT}
                  color={colors.surface}
                />
                {strokes.map(renderStroke)}
                {renderCurrentStroke()}
              </Group>
            </Canvas>
          </GestureDetector>
          {!hasSignature && (
            <View style={styles.placeholderContainer}>
              <Text style={styles.placeholderText}>Sign here</Text>
              <View style={styles.signatureLine} />
            </View>
          )}
        </View>

        {editable && (
          <TouchableOpacity
            style={styles.clearButton}
            onPress={clearSignature}
            disabled={!hasSignature}
          >
            <Text style={[styles.clearButtonText, !hasSignature && styles.disabledText]}>
              Clear
            </Text>
          </TouchableOpacity>
        )}
      </View>
    );
  }
);

SignaturePad.displayName = 'SignaturePad';

const styles = StyleSheet.create({
  container: {
    width: '100%',
  },
  canvasContainer: {
    width: CANVAS_WIDTH,
    height: CANVAS_HEIGHT,
    borderWidth: 1,
    borderColor: colors.border,
    borderRadius: 8,
    backgroundColor: colors.surface,
    overflow: 'hidden',
    alignSelf: 'center',
    position: 'relative',
  },
  disabled: {
    opacity: 0.6,
  },
  canvas: {
    width: CANVAS_WIDTH,
    height: CANVAS_HEIGHT,
    backgroundColor: 'transparent',
  },
  placeholderContainer: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    justifyContent: 'center',
    alignItems: 'center',
    pointerEvents: 'none',
  },
  placeholderText: {
    color: colors.textSecondary,
    fontSize: 16,
    marginBottom: spacing.xl,
  },
  signatureLine: {
    width: CANVAS_WIDTH * 0.6,
    height: 1,
    backgroundColor: colors.border,
  },
  clearButton: {
    marginTop: spacing.sm,
    alignSelf: 'center',
    paddingVertical: spacing.xs,
    paddingHorizontal: spacing.md,
  },
  clearButtonText: {
    color: colors.error,
    fontSize: 14,
    fontWeight: '500',
  },
  disabledText: {
    color: colors.textSecondary,
  },
});

export default SignaturePad;
export { SignaturePadProps };
