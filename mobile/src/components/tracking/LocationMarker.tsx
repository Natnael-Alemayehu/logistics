import React, { memo, useEffect, useRef } from 'react';
import { View, StyleSheet, Animated, Easing } from 'react-native';
import { MARKER_COLORS } from '@/types/maps';
import { colors } from '@/utils/theme';
import { isMapAvailable, getMapLibre } from '@/services/maps/native';

interface LocationMarkerProps {
  id: string;
  coordinate: {
    latitude: number;
    longitude: number;
  };
  heading?: number;
  accuracy?: number;
  speed?: number;
  isTracking?: boolean;
  showAccuracyCircle?: boolean;
  isSelected?: boolean;
  onPress?: () => void;
}

function LocationMarkerComponent({
  id,
  coordinate,
  heading,
  accuracy = 10,
  speed,
  isTracking = false,
  showAccuracyCircle = true,
  isSelected = false,
  onPress,
}: LocationMarkerProps) {
  const MapLibreGL = getMapLibre();
  const pulseAnim = useRef(new Animated.Value(1)).current;
  const opacityAnim = useRef(new Animated.Value(0.3)).current;

  useEffect(() => {
    if (isTracking) {
      Animated.loop(
        Animated.sequence([
          Animated.parallel([
            Animated.timing(pulseAnim, {
              toValue: 2,
              duration: 1000,
              easing: Easing.out(Easing.ease),
              useNativeDriver: true,
            }),
            Animated.timing(opacityAnim, {
              toValue: 0,
              duration: 1000,
              easing: Easing.out(Easing.ease),
              useNativeDriver: true,
            }),
          ]),
          Animated.parallel([
            Animated.timing(pulseAnim, {
              toValue: 1,
              duration: 0,
              useNativeDriver: true,
            }),
            Animated.timing(opacityAnim, {
              toValue: 0.3,
              duration: 0,
              useNativeDriver: true,
            }),
          ]),
        ])
      ).start();
    } else {
      pulseAnim.setValue(1);
      opacityAnim.setValue(0.3);
    }
  }, [isTracking, pulseAnim, opacityAnim]);

  // If MapLibreGL isn't available, return null
  if (!MapLibreGL || !isMapAvailable()) {
    return null;
  }

  const hasHeading = heading !== undefined && heading !== null && heading > 0;
  const markerColor = isTracking ? MARKER_COLORS.current : colors.gray[400];
  
  const featureCollection: GeoJSON.FeatureCollection = {
    type: 'FeatureCollection',
    features: [
      {
        type: 'Feature',
        properties: {},
        geometry: {
          type: 'Point',
          coordinates: [coordinate.longitude, coordinate.latitude],
        },
      },
    ],
  };

  return (
    <>
      {showAccuracyCircle && accuracy > 0 && (
        <MapLibreGL.ShapeSource
          id={`${id}-accuracy-source`}
          shape={featureCollection}
        >
          <MapLibreGL.CircleLayer
            id={`${id}-accuracy`}
            style={{
              circleRadius: accuracy,
              circleColor: markerColor,
              circleOpacity: 0.15,
              circleStrokeWidth: 0,
            }}
          />
        </MapLibreGL.ShapeSource>
      )}
      
      <MapLibreGL.PointAnnotation
        id={id}
        coordinate={[coordinate.longitude, coordinate.latitude]}
        onSelected={onPress}
        anchor={{ x: 0.5, y: 0.5 }}
      >
        <View style={styles.container}>
          {isTracking && showAccuracyCircle && (
            <Animated.View
              style={[
                styles.pulseRing,
                {
                  transform: [{ scale: pulseAnim }],
                  opacity: opacityAnim,
                },
              ]}
            />
          )}
          
          <View
            style={[
              styles.dot,
              { backgroundColor: markerColor },
              isSelected && styles.dotSelected,
            ]}
          >
            {hasHeading && (
              <View
                style={[
                  styles.headingIndicator,
                  { transform: [{ rotate: `${heading}deg` }] },
                ]}
              />
            )}
          </View>
        </View>
      </MapLibreGL.PointAnnotation>
    </>
  );
}

const styles = StyleSheet.create({
  container: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  pulseRing: {
    position: 'absolute',
    width: 40,
    height: 40,
    borderRadius: 20,
    backgroundColor: MARKER_COLORS.current,
  },
  dot: {
    width: 20,
    height: 20,
    borderRadius: 10,
    borderWidth: 3,
    borderColor: '#fff',
    shadowColor: '#000',
    shadowOffset: { width: 0, height: 2 },
    shadowOpacity: 0.25,
    shadowRadius: 4,
    elevation: 4,
    alignItems: 'center',
    justifyContent: 'center',
  },
  dotSelected: {
    borderWidth: 3,
    borderColor: colors.primary,
  },
  headingIndicator: {
    width: 2,
    height: 12,
    backgroundColor: '#fff',
    position: 'absolute',
    top: -6,
  },
});

export const LocationMarker = memo(LocationMarkerComponent);
