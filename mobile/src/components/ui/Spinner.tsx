import React from 'react';
import { View, ActivityIndicator, StyleSheet, ViewStyle } from 'react-native';
import { colors } from '../../utils/theme';

type SpinnerSize = 'sm' | 'md' | 'lg';
type SpinnerColor = 'primary' | 'secondary' | 'white';

interface SpinnerProps {
  size?: SpinnerSize;
  color?: SpinnerColor;
  fullScreen?: boolean;
  style?: ViewStyle;
}

const Spinner: React.FC<SpinnerProps> = ({
  size = 'md',
  color = 'primary',
  fullScreen = false,
  style,
}) => {
  const getActivityIndicatorSize = (): 'small' | 'large' | number => {
    switch (size) {
      case 'sm':
        return 'small';
      case 'lg':
        return 48;
      case 'md':
      default:
        return 'large';
    }
  };

  const getActivityIndicatorColor = (): string => {
    switch (color) {
      case 'primary':
        return colors.primary;
      case 'secondary':
        return colors.secondary;
      case 'white':
        return '#ffffff';
      default:
        return colors.primary;
    }
  };

  if (fullScreen) {
    return (
      <View style={[styles.fullScreen, style]}>
        <ActivityIndicator
          size={getActivityIndicatorSize()}
          color={getActivityIndicatorColor()}
        />
      </View>
    );
  }

  return (
    <View style={style}>
      <ActivityIndicator
        size={getActivityIndicatorSize()}
        color={getActivityIndicatorColor()}
      />
    </View>
  );
};

const styles = StyleSheet.create({
  fullScreen: {
    position: 'absolute',
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    backgroundColor: 'rgba(255, 255, 255, 0.8)',
    justifyContent: 'center',
    alignItems: 'center',
    zIndex: 9999,
  },
});

export default Spinner;
